package main

import (
	"bytes"
	"fmt"
	"github.com/Ferluci/fast-realip"
	"github.com/technically-functional/heartbeat/templates"
	"github.com/valyala/fasthttp"
	"log"
	"strconv"
	"time"
)

var (
	apiPrefix = []byte("/api/")
	cssPrefix = []byte("/css/")
	icoSuffix = []byte(".ico")
	pngSuffix = []byte(".png")
	gitRepo   = "https://github.com/technically-functional/heartbeat" // set in .env

	cssHandler = fasthttp.FSHandler("www", 1)
	imgHandler = fasthttp.FSHandler("www", 0)
)

func RequestHandler(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set(fasthttp.HeaderServer, serverName)

	path := ctx.Path()
	pathStr := string(path)

	switch {
	case bytes.HasPrefix(path, apiPrefix):
		ApiHandler(ctx, pathStr)
	case bytes.HasPrefix(path, cssPrefix):
		cssHandler(ctx)
	case bytes.HasSuffix(path, icoSuffix), bytes.HasSuffix(path, pngSuffix):
		imgHandler(ctx)
	default:
		totalVisits++
		ctx.Response.Header.Set(fasthttp.HeaderContentType, "text/html; charset=utf-8")

		switch pathStr {
		case "/":
			MainPageHandler(ctx)
		case "/privacy":
			PrivacyPolicyPageHandler(ctx)
		case "/stats":
			StatsPageHandler(ctx)
		default:
			ErrorPageHandler(ctx, fasthttp.StatusNotFound, "404 Not Found")
		}
	}
}

func ApiHandler(ctx *fasthttp.RequestCtx, path string) {
	if !ctx.IsPost() {
		ErrorPageHandler(ctx, fasthttp.StatusBadRequest, "400 Bad Request")
		return
	}

	// The authentication key provided with said Auth header
	header := ctx.Request.Header.Peek("Auth")

	// Make sure Auth key is correct
	if string(header) != authToken {
		ErrorPageHandler(ctx, fasthttp.StatusForbidden, "403 Forbidden")
		return
	}

	switch path {
	case "/api/beat":
		handleSuccessfulBeat(ctx)
	default:
		ErrorPageHandler(ctx, fasthttp.StatusBadRequest, "400 Bad Request")
	}
}

func MainPageHandler(ctx *fasthttp.RequestCtx) {
	p := getMainPage()
	templates.WritePageTemplate(ctx, p)
}

func PrivacyPolicyPageHandler(ctx *fasthttp.RequestCtx) {
	p := &templates.PrivacyPolicyPage{
		ServerName: serverName,
	}
	templates.WritePageTemplate(ctx, p)
}

func StatsPageHandler(ctx *fasthttp.RequestCtx) {
	p := &templates.StatsPage{
		TotalBeats:   FormattedNum(totalBeats),
		TotalDevices: FormattedNum(2), // TODO: Add support for this
		TotalVisits:  FormattedNum(totalVisits),
		ServerName:   serverName,
	}
	templates.WritePageTemplate(ctx, p)
}

func ErrorPageHandler(ctx *fasthttp.RequestCtx, code int, message string) {
	p := &templates.ErrorPage{
		Message: message,
		Path:    ctx.Path(),
		Method:  ctx.Method(),
	}
	templates.WritePageTemplate(ctx, p)
	ctx.SetStatusCode(code)
	log.Printf("- Returned %v to %s - tried to connect with '%s'", code, realip.FromRequest(ctx), ctx.Method())
}

func getMainPage() *templates.MainPage {
	currentTime := time.Now()
	currentBeatDifference := currentTime.Unix() - lastBeat

	// We also want to live update the current difference, instead of only when receiving a beat.
	if currentBeatDifference > missingBeat {
		missingBeat = currentBeatDifference
	}

	lastSeen := time.Unix(lastBeat, 0).Format(timeFormat)
	timeDifference := TimeDifference(lastBeat, currentTime)
	missingBeatFmt := FormattedTime(missingBeat)
	totalBeatsFmt := FormattedNum(totalBeats)
	currentTimeStr := currentTime.Format(timeFormat)

	page := &templates.MainPage{
		LastSeen:       lastSeen,
		TimeDifference: timeDifference,
		MissingBeat:    missingBeatFmt,
		TotalBeats:     totalBeatsFmt,
		CurrentTime:    currentTimeStr,
		GitHash:        gitCommitHash,
		GitRepo:        gitRepo,
		ServerName:     serverName,
	}

	return page
}

func handleSuccessfulBeat(ctx *fasthttp.RequestCtx) {
	totalBeats += 1
	newLastBeat := time.Now().Unix()
	currentBeatDifference := newLastBeat - lastBeat

	if currentBeatDifference > missingBeat {
		missingBeat = currentBeatDifference
	}

	lastBeatStr := strconv.FormatInt(newLastBeat, 10)
	missingBeatStr := strconv.FormatInt(missingBeat, 10)
	totalBeatsStr := strconv.FormatInt(totalBeats, 10)

	fmt.Fprintf(ctx, "%v\n", lastBeatStr)
	log.Printf("- Successful beat from %s", realip.FromRequest(ctx))

	lastBeat = newLastBeat
	WriteToFile("config/last_beat", lastBeatStr+":"+missingBeatStr+":"+totalBeatsStr)
	WriteGetRequestsFile(totalVisits)
}
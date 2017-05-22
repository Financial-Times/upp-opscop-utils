package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/jawher/mow.cli"
	stdoutlog "log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	idListFilePath  *string
	successFilePath *string
	failedFilePath  *string
	retryFilePath   *string
	validationUrl   *string
	auth            *string

	success *os.File
	fail    *os.File
	retry   *os.File
)

func main() {

	app := cli.App("publishing-checker", "script that takes a list of uuids and validates they are in UPP")
	idListFilePath = app.StringOpt("idListFile", "", "uuids to check")
	successFilePath = app.StringOpt("successFile", "success.txt", "uuids found successfully")
	failedFilePath = app.StringOpt("failedFile", "failed.txt", "uuids not found")
	retryFilePath = app.StringOpt("retryFile", "retry.txt", "uuids republished")
	validationUrl = app.StringOpt("validationUrl", "", "The endpoint to validate against")
	auth = app.StringOpt("auth", "", "Authorization header value")

	app.Action = func() {
		f, err := os.OpenFile("publishing-checker.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err == nil {
			customFormatter := new(log.TextFormatter)
			customFormatter.TimestampFormat = "2006-01-02 15:04:05"
			customFormatter.FullTimestamp = true
			log.SetFormatter(customFormatter)
			log.SetOutput(f)
		} else {
			log.Fatalf("Failed to initialise log file, %v", err)
		}

		defer f.Close()
		contentPresentValidator()
	}

	log.SetLevel(log.InfoLevel)
	stdoutlog.Printf("Application started with args %s", os.Args)
	app.Run(os.Args)
}

func contentPresentValidator() {
	stdoutlog.Printf("validating ids in file")

	lines := make(chan string, 250000)
	go readLines(*idListFilePath, lines)

	go func() {
		t := time.NewTicker(1 * time.Second)
		for {
			<-t.C
			stdoutlog.Printf("remaining: %d\n", len(lines))
			log.Printf("remaining: %d\n", len(lines))
		}
	}()

	succeeded := make(chan string)
	retry := make(chan string)
	failed := make(chan string)

	var httpWG sync.WaitGroup
	for i := 0; i < 4; i++ {
		httpWG.Add(1)
		go func() {
			defer httpWG.Done()
			for line := range lines {
				const layout = "2006-01-02T15:04:05.000+0000"
				split := strings.Split(line, ",")
				alertTime, _ := time.Parse(layout, split[0])
				stdoutlog.Printf("time:" + alertTime.String())
				sendRequestToCheck(split[1], alertTime, succeeded, retry, failed)
			}
		}()
	}

	go func() {
		httpWG.Wait()
		close(succeeded)
		close(retry)
		close(failed)
	}()

	var wg sync.WaitGroup

	wg.Add(3)
	go writeoutAll(succeeded, *successFilePath, &wg)
	go writeoutAll(failed, *failedFilePath, &wg)
	go writeoutAll(retry, *retryFilePath, &wg)
	wg.Wait()

	stdoutlog.Printf("Done")

}

func readLines(path string, lines chan<- string) {
	stdoutlog.Printf("Reading lines")

	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines <- scanner.Text()
	}
	close(lines)
}

func writeoutAll(all <-chan string, filename string, wg *sync.WaitGroup) {
	defer wg.Done()
	f, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	bw := bufio.NewWriter(f)
	defer bw.Flush()
	for s := range all {
		bw.WriteString(fmt.Sprintf("%s\n", s))
	}
}

var client = http.Client{Timeout: time.Duration(5 * time.Second),
	Transport: &http.Transport{DisableKeepAlives: false, MaxIdleConnsPerHost: 30}}

func sendRequestToCheck(uuid string, alertTime time.Time, succeeded, retry, failed chan string) {

	stdoutlog.Printf("About to check uuid : " + uuid)

	requestUrl := *validationUrl + uuid
	stdoutlog.Printf(requestUrl)

	req, err := http.NewRequest("GET", requestUrl, nil)
	req.Header.Set("X-Request-Id", "SYNTHETIC-REQ-MON-NETWORK-Validator")
	req.Header.Set("Authorization", *auth)
	resp, err := client.Do(req)

	if err != nil {
		retry <- uuid
	}

	var data ContentItem
	json.NewDecoder(resp.Body).Decode(&data)

	lastUppPublish := data.LastModified

	const responseDatelayout = "2006-01-02T15:04:05.000Z"

	if resp.StatusCode == http.StatusOK && len(lastUppPublish) > 0 {
		lastUppPubDate, _ := time.Parse(responseDatelayout, lastUppPublish)
		if alertTime.Before(lastUppPubDate) {
			succeeded <- uuid + "    " + lastUppPubDate.String()
		} else {
			retry <- uuid
		}
	} else {
		failed <- uuid
	}

}

type ContentItem struct {
	LastModified string
}

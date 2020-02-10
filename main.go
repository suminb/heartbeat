package main

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

type LogEvent struct {
	msg string
	// Timestamp in milliseconds
	timestamp int64
}

// TODO: Avoid using hard-coded values
const logGroup = "test"
const logStream = "stream1"

var token *string

func main() {
	cloudwatch()
}

func cloudwatch() {
	// Initial credentials loaded from SDK's default credential chain. Such as
	// the environment, shared credentials (~/.aws/credentials), or EC2 Instance
	// Role. These credentials will be used to to make the STS Assume Role API.
	sess := session.Must(session.NewSession())

	svc := cloudwatchlogs.New(sess, aws.NewConfig().WithRegion("ap-northeast-2"))

	events := make([]LogEvent, 1)
	events[0] = LogEvent{
		msg:       getNetworkInfo(),
		timestamp: makeTimestamp(),
	}

	uploadLogs(svc, events)
}

func uploadLogs(svc *cloudwatchlogs.CloudWatchLogs, events []LogEvent) {
	setToken(svc)

	logevents := make([]*cloudwatchlogs.InputLogEvent, 0, len(events))
	for _, elem := range events {
		logevents = append(logevents, &cloudwatchlogs.InputLogEvent{
			Message:   aws.String(elem.msg),
			Timestamp: aws.Int64(elem.timestamp),
		})
	}
	params := &cloudwatchlogs.PutLogEventsInput{
		LogEvents:     logevents,
		LogGroupName:  aws.String(logGroup),
		LogStreamName: aws.String(logStream),
		SequenceToken: token,
	}

	// When rejectedLogEventsInfo is not empty, app can not
	// do anything reasonable with rejected logs. Ignore it.
	// Meybe expose some statistics for rejected counters.
	resp, err := svc.PutLogEvents(params)
	if err != nil {
		panic(err)
	}

	fmt.Printf("resp = %v\n", resp)
}

func setToken(svc *cloudwatchlogs.CloudWatchLogs) error {
	params := &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        aws.String(logGroup),
		LogStreamNamePrefix: aws.String(logStream),
	}

	return svc.DescribeLogStreamsPages(params,
		func(page *cloudwatchlogs.DescribeLogStreamsOutput, lastPage bool) bool {
			return !findToken(page)
		})
}

func findToken(page *cloudwatchlogs.DescribeLogStreamsOutput) bool {
	fmt.Printf("Found %d log streams\n", len(page.LogStreams))
	for _, row := range page.LogStreams {
		if logStream == *row.LogStreamName {
			token = row.UploadSequenceToken
			return true
		}
	}
	return false
}

func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func getNetworkInfo() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}

	var builder strings.Builder

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			panic(err)
		}
		builder.WriteString(fmt.Sprintf("interface %s:\n", i.Name))
		builder.WriteString(fmt.Sprintf("  hardware addr: %s\n", i.HardwareAddr))

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			builder.WriteString(fmt.Sprintf("  ip: %s\n", ip))
		}
	}

	return builder.String()
}

package main

import (
	"context"
	"fmt"
	"github.com/google/go-github/v38/github"
	"github.com/willabides/ezactions"
	"golang.org/x/oauth2"
	"log"
	"os"
	"regexp"
	"strconv"
	"time"
)

//go:generate go run . -generate

var action = &ezactions.Action{
	Name:        "Register PR",
	Description: "Register a merged PR for tracking",
	Inputs:      []ezactions.ActionInput{},
	Outputs:     []ezactions.ActionOutput{},
	Run:         actionMain,
}

func main() {
	action.Main()
}

func getEnvOrDefault(key string, def string) string {
	envValue := os.Getenv(key)
	if len(envValue) > 0 {
		return envValue
	}
	return def
}

var token = os.Getenv("GITHUB_TOKEN")
var eventPullRequestTitle = os.Getenv("EVENT_PR_TITLE")
var eventPullRequestIssuer = os.Getenv("EVENT_PR_ISSUER")
var eventPullRequestNumber = os.Getenv("EVENT_PR_NUMBER")
var eventPullRequestBody = os.Getenv("EVENT_PR_BODY")
var mergedPullRequestNumber = os.Getenv("MERGED_PR_NUMBER")

var organiziation = getEnvOrDefault("ORGANIZATION", "apiiro")
var trackedRepo = getEnvOrDefault("TRACKED_REPO", "lim")
var trackingRepo = getEnvOrDefault("TRACKING_REPO", "lim-tracking")

func actionMain(_ map[string]string, _ *ezactions.RunResources) (map[string]string, error) {

	var pullRequestNumber = eventPullRequestNumber
	if len(pullRequestNumber) == 0 {
		pullRequestNumber = mergedPullRequestNumber
	}
	if len(pullRequestNumber) == 0 {
		return nil, fmt.Errorf("a valid pull request number was not provided")
	}
	if len(token) == 0 {
		return nil, fmt.Errorf("a valid token was not provided")
	}

	pullRequestIntNumber, err := strconv.Atoi(pullRequestNumber)
	if err != nil {
		return nil, fmt.Errorf("a valid pull request number was not provided (%v)", pullRequestNumber)
	}

	log.Printf("working with pull request number %v", pullRequestNumber)

	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	ctx := context.Background()
	oauthClient := oauth2.NewClient(ctx, tokenSource)
	githubClient := github.NewClient(oauthClient)

	pullRequestTitle := eventPullRequestTitle
	pullRequestIssuer := eventPullRequestIssuer
	pullRequestBody := pullRequestBodySanitizer(eventPullRequestBody)

	if len(pullRequestTitle) == 0 {
		pr, _, err := githubClient.PullRequests.Get(ctx, organiziation, trackedRepo, pullRequestIntNumber)
		if err == nil {
			pullRequestIssuer = pr.GetUser().GetLogin()
			pullRequestTitle = pr.GetTitle()
		} else {
			log.Printf("failed to find pr title and issuer for %v: %v", pullRequestNumber, err)
		}
	}

	response, err := createFile(pullRequestTitle, pullRequestIssuer, pullRequestNumber, pullRequestBody, githubClient, ctx)
	if response != nil && response.StatusCode == 422 {
		log.Printf("file already exists for %v: %v, considering as success", pullRequestNumber, err)
		return map[string]string{}, nil
	}
	if err != nil {
		if _, ok := err.(*github.RateLimitError); ok {
			waitDuration := time.Since(response.Rate.Reset.Time)
			log.Printf("hit rate limit, will need to wait %v sec and retry", waitDuration.Seconds())
			time.Sleep(waitDuration)
		} else {
			return nil, fmt.Errorf("failed to create file for %v: %v", pullRequestNumber, err)
		}
	}
	if response.StatusCode == 201 {
		log.Printf("completed successfully!")
		return map[string]string{}, nil
	}
	return nil, fmt.Errorf("unexpected non-error status for %v: %v", pullRequestNumber, response.StatusCode)
}

func createFile(eventPullRequestTitle string, eventPullRequestIssuer string, pullRequestNumber string, pullRequestBody string, githubClient *github.Client, ctx context.Context) (*github.Response, error) {
	committer := "github-actions"
	committerEmail := "github-actions@github.com"
	message := fmt.Sprintf("Automated marker set in place by closing pull request #%v", pullRequestNumber)
	now := time.Now()
	author := &github.CommitAuthor{
		Date:  &now,
		Name:  &committer,
		Email: &committerEmail,
		Login: &committer,
	}
	fileContent := fmt.Sprintf("%v\n%v\n%v\n%v", pullRequestNumber, eventPullRequestTitle, eventPullRequestIssuer, pullRequestBody)
	_, response, err := githubClient.Repositories.CreateFile(
		ctx,
		organiziation,
		trackingRepo,
		fmt.Sprintf("terminal/%v.marker", pullRequestNumber),
		&github.RepositoryContentFileOptions{
			Message:   &message,
			Content:   []byte(fileContent),
			Author:    author,
			Committer: author,
		},
	)
	return response, err
}

func pullRequestBodySanitizer(pullRequestBody string) string {
	if len(pullRequestBody) == 0 {
		return ""
	}

	regexPattern := regexp.MustCompile(`(?i)\b(?:close|closes):?\b.*?\b(LIM-\d+)\b`)
	matches := regexPattern.FindAllStringSubmatch(pullRequestBody, -1)

	var tickets string
	for _, match := range matches {
		tickets += match[1] + " "
	}

	return tickets
}

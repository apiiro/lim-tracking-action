package main

import (
	"context"
	"fmt"
	"golang.org/x/oauth2"
	"log"
	"os"
	"time"

	"github.com/google/go-github/v38/github"
	"github.com/willabides/ezactions"
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

var token = os.Getenv("GITHUB_TOKEN")
var eventPullRequestNumber = os.Getenv("EVENT_PR_NUMBER")
var mergedPullRequestNumber = os.Getenv("MERGED_PR_NUMBER")

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

	log.Printf("working with pull request number %v", pullRequestNumber)

	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	ctx := context.Background()
	oauthClient := oauth2.NewClient(ctx, tokenSource)
	githubClient := github.NewClient(oauthClient)

	for {
		response, err := createFile(pullRequestNumber, githubClient, ctx)
		if response != nil && response.StatusCode == 422 {
			log.Printf("file already exists for %v: %v, considering as success", pullRequestNumber, err)
			return map[string]string{}, nil
		}
		if err != nil {
			if _, ok := err.(*github.RateLimitError); ok {
				waitDuration := time.Now().Sub(response.Rate.Reset.Time)
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
}

func createFile(pullRequestNumber string, githubClient *github.Client, ctx context.Context) (*github.Response, error) {
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
	_, response, err := githubClient.Repositories.CreateFile(
		ctx,
		"apiiro",
		"lim-tracking",
		fmt.Sprintf("terminal/%v.marker", pullRequestNumber),
		&github.RepositoryContentFileOptions{
			Message:   &message,
			Content:   []byte(pullRequestNumber),
			Author:    author,
			Committer: author,
		},
	)
	return response, err
}

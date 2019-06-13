package external

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"gopkg.in/google/go-github.v15/github"
)

// GitHubMatcher matches emails and GitHub users.
type GitHubMatcher struct {
	client *github.Client
}

// NewGitHubMatcher creates a new matcher given a GitHub token.
// https://github.com/settings/tokens
func NewGitHubMatcher(apiURL, token string) (Matcher, error) {
	if apiURL == "" {
		apiURL = "https://api.github.com/"
	}
	var c *http.Client
	if token != "" {
		c = oauth2.NewClient(
			context.Background(),
			oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}),
		)
	}
	// The actual upload URL does not matter - we are not going to upload anything.
	client, err := github.NewEnterpriseClient(apiURL, apiURL, c)
	if err != nil {
		return GitHubMatcher{}, err
	}
	return GitHubMatcher{client}, nil
}

var searchOpts = &github.SearchOptions{
	Sort:        "joined",
	ListOptions: github.ListOptions{PerPage: 1},
}

// MatchByEmail returns the latest GitHub user with the given email.
func (m GitHubMatcher) MatchByEmail(ctx context.Context, email string) (user, name string, err error) {
	finished := make(chan struct{})
	go func() {
		defer func() { finished <- struct{}{} }()
		query := email + " in:email"
		for { // api rate limit retry loop
			if isNoReplyEmail(email) {
				user = userFromEmail(email)
			} else {
				var result *github.UsersSearchResult
				result, _, err = m.client.Search.Users(ctx, query, searchOpts)
				if err != nil {
					if rl, ok := err.(*github.RateLimitError); ok {
						logRateLimitError(rl)
						time.Sleep(rl.Rate.Reset.Sub(time.Now().UTC()))
						continue
					}
					return
				}
				if len(result.Users) == 0 {
					if strings.Contains(query, "@") {
						// Hacking time! user+domain may work instead of user@domain
						query = strings.Replace(query, "@", " ", 1)
						continue
					}
					logrus.Warnf("unable to find users for email: %s", email)
					err = ErrNoMatches
					return
				}
				user = result.Users[0].GetLogin()
				break
			}
		}

		for { // api rate limit retry loop
			var u *github.User
			u, _, err = m.client.Users.Get(ctx, user)
			if err != nil {
				if rl, ok := err.(*github.RateLimitError); ok {
					logRateLimitError(rl)
					time.Sleep(rl.Rate.Reset.Sub(time.Now().UTC()))
					continue
				}
				return
			}
			user = u.GetLogin()
			name = u.GetName()
			return
		}
	}()
	select {
	case <-finished:
		return
	case <-ctx.Done():
		return "", "", context.Canceled
	}
}

func isNoReplyEmail(email string) bool {
	return strings.HasSuffix(email, "@users.noreply.github.com")
}

func userFromEmail(email string) string {
	user := strings.Split(email, "@")[0]

	// Some emails can be of the form xxxxx+yyyyyy@users.noreply.github.com
	if strings.Contains(user, "+") {
		user = strings.Split(user, "+")[1]
	}

	return user
}

func logRateLimitError(rl *github.RateLimitError) {
	logrus.Warnf("rate limit was hit, waiting until %s", rl.Rate.Reset)
}

package external

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
)

// GitLabMatcher matches emails and GitLab users.
type GitLabMatcher struct {
	client *gitlab.Client
}

// NewGitLabMatcher creates a new matcher given a GitLab OAuth token.
// https://gitlab.com/profile/personal_access_tokens
func NewGitLabMatcher(apiURL, token string) (Matcher, error) {
	if apiURL == "" {
		apiURL = "https://gitlab.com/api/v4"
	}
	m := GitLabMatcher{gitlab.NewClient(nil, token)}
	err := m.client.SetBaseURL(apiURL)
	if err != nil {
		return GitLabMatcher{}, err
	}
	return m, nil
}

// MatchByEmail returns the latest GitLab user with the given email.
func (m GitLabMatcher) MatchByEmail(ctx context.Context, email string) (user string, err error) {
	finished := make(chan struct{})
	go func() {
		defer func() { finished <- struct{}{} }()
		opts := &gitlab.ListUsersOptions{Search: &email}
		for {
			var users []*gitlab.User
			users, _, err = m.client.Users.ListUsers(opts)
			if err != nil {
				// TODO(vmarkovtsev): handle rate limit
				// https://github.com/xanzy/go-gitlab/issues/630
				return
			}
			if len(users) == 0 {
				logrus.Warnf("unable to find users for email: %s", email)
				err = ErrNoMatches
				return
			}
			user = users[0].Username
			// name = users[0].Name
			return
		}
	}()
	select {
	case <-finished:
		return
	case <-ctx.Done():
		return "", context.Canceled
	}
}

// SupportsMatchingByCommit indicates whether this Matcher allows querying identities by commit metadata.
func (m GitLabMatcher) SupportsMatchingByCommit() bool {
	return false
}

// MatchByCommit queries the identity of a given email address in a particular commit context.
func (m GitLabMatcher) MatchByCommit(
	ctx context.Context, email, repo, commit string) (user string, err error) {
	return "", errors.New("not implemented")
}

// OnIdle does nothing here.
func (m GitLabMatcher) OnIdle() error {
	return nil
}

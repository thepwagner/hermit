package hooks

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/go-redis/redis/v8"
	"github.com/google/go-github/v43/github"
)

// Server listens to GitHub webhooks and dispatches work requests to the redis queue.
type Server struct {
	log           logr.Logger
	redis         *redis.Client
	gh            *github.Client
	botID         int64
	webhookSecret []byte
}

func NewServer(log logr.Logger, redis *redis.Client, gh *github.Client, botID int64, webhookSecret []byte) *Server {
	return &Server{
		log:           log,
		redis:         redis,
		gh:            gh,
		botID:         botID,
		webhookSecret: webhookSecret,
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, s.webhookSecret)
	if err != nil {
		s.log.Info("invalid payload", "path", r.URL.Path)
		http.NotFound(w, r)
		return
	}

	switch hook := github.WebHookType(r); hook {
	case "push":
		err = s.OnPush(r, payload)
	default:
		s.log.Info("ignored event", "event", hook)
	}

	if err != nil {
		s.log.Error(err, "failed to handle event")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
}

func (s *Server) OnPush(r *http.Request, payload []byte) error {
	// Parse and extract features:
	evt, err := github.ParseWebHook("push", payload)
	if err != nil {
		return err
	}
	pushEvt := evt.(*github.PushEvent)
	repo := pushEvt.GetRepo()
	repoOwner := repo.GetOwner().GetLogin()
	repoName := repo.GetName()
	ref := pushEvt.GetRef()
	sha := pushEvt.GetAfter()

	// Create a queued checkrun for anything that will be built:
	if sha == "0000000000000000000000000000000000000000" {
		s.log.Info("ignoring delete event", "owner", repoOwner, "repo", repoName, "ref", ref)
		return nil
	}
	s.log.Info("received push event", "owner", repoOwner, "repo", repoName, "sha", sha, "ref", ref)
	buildCheckRun, _, err := s.gh.Checks.CreateCheckRun(r.Context(), repoOwner, repoName, github.CreateCheckRunOptions{
		Name:    buildCheckRunName,
		Status:  github.String("queued"),
		HeadSHA: sha,
	})
	if err != nil {
		return err
	}

	b, err := json.Marshal(&BuildRequest{
		RepoOwner:       repoOwner,
		RepoName:        repoName,
		SHA:             sha,
		Tree:            pushEvt.GetHeadCommit().GetTreeID(),
		Ref:             ref,
		BuildCheckRunID: buildCheckRun.GetID(),
		DefaultBranch:   fmt.Sprintf("refs/heads/%s", repo.GetDefaultBranch()),
		FromHermit:      pushEvt.GetSender().GetID() == s.botID,
	})
	if err != nil {
		return err
	}
	return s.redis.RPush(r.Context(), buildRequestQueue, b).Err()
}

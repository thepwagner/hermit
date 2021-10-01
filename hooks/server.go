package hooks

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/go-redis/redis/v8"
	"github.com/google/go-github/v39/github"
)

type Server struct {
	log           logr.Logger
	redis         *redis.Client
	gh            *github.Client
	webhookSecret []byte
}

func NewServer(log logr.Logger, redis *redis.Client, gh *github.Client, webhookSecret []byte) *Server {
	return &Server{
		log:           log,
		redis:         redis,
		gh:            gh,
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
	s.log.Info("received push event event", len(payload))
	evt, err := github.ParseWebHook("push", payload)
	if err != nil {
		return err
	}
	pushEvt := evt.(*github.PushEvent)

	repo := pushEvt.GetRepo()
	repoOwner := repo.GetOwner().GetLogin()
	repoName := repo.GetName()
	buildCheckRun, _, err := s.gh.Checks.CreateCheckRun(r.Context(), repoOwner, repoName, github.CreateCheckRunOptions{
		Name:    buildCheckRunName,
		Status:  github.String("queued"),
		HeadSHA: pushEvt.GetAfter(),
	})
	if err != nil {
		return err
	}

	b, err := json.Marshal(&BuildRequest{
		RepoOwner:       repoOwner,
		RepoName:        repoName,
		SHA:             pushEvt.GetAfter(),
		Tree:            pushEvt.GetHeadCommit().GetTreeID(),
		Ref:             pushEvt.GetRef(),
		BuildCheckRunID: buildCheckRun.GetID(),
		DefaultBranch:   pushEvt.GetRef() == fmt.Sprintf("refs/heads/%s", repo.GetDefaultBranch()),
	})
	if err != nil {
		return err
	}
	return s.redis.RPush(r.Context(), buildRequestQueue, b).Err()
}

const buildRequestQueue = "github-hook-build"

type BuildRequest struct {
	RepoOwner       string
	RepoName        string
	Ref             string
	SHA             string
	Tree            string
	BuildCheckRunID int64
	DefaultBranch   bool
}

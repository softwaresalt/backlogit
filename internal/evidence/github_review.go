package evidence

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// GitHub derives PR evidence through the gh CLI.
type GitHub struct {
	// Runner executes gh. Defaults to ExecRunner when nil.
	Runner Runner
	// Repo is the owner/name repository slug.
	Repo string
	// Token is passed to gh only through the GH_TOKEN environment variable.
	// When empty gh uses its own ambient authentication.
	Token string
}

func (g *GitHub) runner() Runner {
	if g.Runner == nil {
		return ExecRunner{}
	}
	return g.Runner
}

func (g *GitHub) env() []string {
	if g.Token == "" {
		return nil
	}
	return []string{"GH_TOKEN=" + g.Token}
}

// ghRun runs gh and returns stdout. A failure yields a scrubbed reason.
func (g *GitHub) ghRun(ctx context.Context, what string, args ...string) ([]byte, string) {
	if err := ctx.Err(); err != nil {
		return nil, degraded(what, err, nil, g.Token)
	}
	out, stderr, err := g.runner().Run(ctx, "gh", args, g.env(), nil)
	if err != nil {
		return nil, degraded(what, err, stderr, g.Token)
	}
	return out, ""
}

func splitRepo(slug string) (owner, repo string, ok bool) {
	owner, repo, found := strings.Cut(slug, "/")
	return owner, repo, found && owner != "" && repo != "" && !strings.Contains(repo, "/")
}

// ReviewResult is the derived GitHub review evidence for one PR.
type ReviewResult struct {
	Status Status
	Reason string
	PR     int
	// HeadSHA is the PR's current head commit.
	HeadSHA string
	// ReviewedHeads lists the distinct commits that submitted reviews cover.
	ReviewedHeads []string
	// ReviewedCurrent is true when a submitted review covers HeadSHA.
	ReviewedCurrent bool
	// ReviewCycles counts submitted (non-pending) reviews.
	ReviewCycles      int
	ReviewRequests    []string
	UnresolvedThreads int
}

const reviewQuery = `query($owner:String!,$repo:String!,$pr:Int!,$cursor:String){
repository(owner:$owner,name:$repo){pullRequest(number:$pr){
headRefOid
reviewRequests(first:100){nodes{requestedReviewer{__typename ... on User{login} ... on Bot{login} ... on Team{name}}}}
reviews(last:100){nodes{state commit{oid}}}
reviewThreads(first:100,after:$cursor){nodes{isResolved} pageInfo{hasNextPage endCursor}}}}}`

type reviewPayload struct {
	Data struct {
		Repository struct {
			PullRequest *struct {
				HeadRefOid     string `json:"headRefOid"`
				ReviewRequests struct {
					Nodes []struct {
						RequestedReviewer struct {
							Login string `json:"login"`
							Name  string `json:"name"`
						} `json:"requestedReviewer"`
					} `json:"nodes"`
				} `json:"reviewRequests"`
				Reviews struct {
					Nodes []struct {
						State  string `json:"state"`
						Commit struct {
							Oid string `json:"oid"`
						} `json:"commit"`
					} `json:"nodes"`
				} `json:"reviews"`
				ReviewThreads struct {
					Nodes []struct {
						IsResolved bool `json:"isResolved"`
					} `json:"nodes"`
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
				} `json:"reviewThreads"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

// maxThreadPages bounds thread pagination so a misbehaving API cannot loop.
const maxThreadPages = 50

// ReviewEvidence derives reviewed HEADs, review cycles/requests and unresolved
// thread counts for a PR. Any failure yields StatusDegraded, never a pass, and
// a degraded result carries no partial evidence.
func (g *GitHub) ReviewEvidence(ctx context.Context, pr int) ReviewResult {
	fail := func(reason string) ReviewResult {
		return ReviewResult{Status: StatusDegraded, Reason: reason, PR: pr}
	}
	owner, repo, ok := splitRepo(g.Repo)
	if !ok {
		return fail(fmt.Sprintf("invalid repository slug %q", g.Repo))
	}

	res := ReviewResult{PR: pr}
	cursor := ""
	for page := 0; page < maxThreadPages; page++ {
		args := []string{"api", "graphql",
			"-f", "query=" + reviewQuery,
			"-f", "owner=" + owner, "-f", "repo=" + repo,
			"-F", "pr=" + strconv.Itoa(pr)}
		if cursor != "" {
			args = append(args, "-f", "cursor="+cursor)
		}
		out, reason := g.ghRun(ctx, "gh review query", args...)
		if reason != "" {
			return fail(reason)
		}
		var p reviewPayload
		if err := json.Unmarshal(out, &p); err != nil {
			return fail(degraded("decode review payload", err, nil, g.Token))
		}
		prd := p.Data.Repository.PullRequest
		if prd == nil || prd.HeadRefOid == "" {
			return fail(fmt.Sprintf("pull request %d not found", pr))
		}
		if page == 0 {
			res.HeadSHA = prd.HeadRefOid
			seen := map[string]bool{}
			for _, r := range prd.Reviews.Nodes {
				if r.State == "PENDING" {
					continue
				}
				res.ReviewCycles++
				if oid := r.Commit.Oid; oid != "" && !seen[oid] {
					seen[oid] = true
					res.ReviewedHeads = append(res.ReviewedHeads, oid)
				}
			}
			res.ReviewedCurrent = seen[res.HeadSHA]
			for _, rr := range prd.ReviewRequests.Nodes {
				name := rr.RequestedReviewer.Login
				if name == "" {
					name = rr.RequestedReviewer.Name
				}
				if name != "" {
					res.ReviewRequests = append(res.ReviewRequests, name)
				}
			}
		}
		for _, th := range prd.ReviewThreads.Nodes {
			if !th.IsResolved {
				res.UnresolvedThreads++
			}
		}
		if !prd.ReviewThreads.PageInfo.HasNextPage {
			res.Status = StatusVerified
			return res
		}
		cursor = prd.ReviewThreads.PageInfo.EndCursor
	}
	return fail("review thread pagination exceeded page limit")
}

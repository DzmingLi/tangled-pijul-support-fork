package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	_ "github.com/mattn/go-sqlite3"
)

var (
	dbPath = flag.String("db", "appview.db", "Path to SQLite database")
	count  = flag.Int("count", 10, "Number of pipeline runs to generate")
	repo   = flag.String("repo", "", "Repository name (e.g., 'did:plc:xyz/myrepo')")
	knot   = flag.String("knot", "localhost:8100", "Knot hostname")
)

// StatusKind represents the status of a workflow
type StatusKind string

const (
	StatusKindPending   StatusKind = "pending"
	StatusKindRunning   StatusKind = "running"
	StatusKindFailed    StatusKind = "failed"
	StatusKindTimeout   StatusKind = "timeout"
	StatusKindCancelled StatusKind = "cancelled"
	StatusKindSuccess   StatusKind = "success"
)

var finishStatuses = []StatusKind{
	StatusKindFailed,
	StatusKindTimeout,
	StatusKindCancelled,
	StatusKindSuccess,
}

// generateRandomSha generates a random 40-character SHA
func generateRandomSha() string {
	const hexChars = "0123456789abcdef"
	sha := make([]byte, 40)
	for i := range sha {
		sha[i] = hexChars[rand.Intn(len(hexChars))]
	}
	return string(sha)
}

// generateRkey generates a TID-like rkey
func generateRkey() string {
	// Simple timestamp-based rkey
	now := time.Now().UnixMicro()
	return fmt.Sprintf("%d", now)
}

func main() {
	flag.Parse()

	if *repo == "" {
		log.Fatal("--repo is required (format: did:plc:xyz/reponame)")
	}

	// Parse repo into owner and name
	did, repoName, ok := parseRepo(*repo)
	if !ok {
		log.Fatalf("Invalid repo format: %s (expected: did:plc:xyz/reponame)", *repo)
	}

	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	rand.Seed(time.Now().UnixNano())

	branches := []string{"main", "develop", "feature/auth", "fix/bugs"}
	workflows := []string{"test", "build", "lint", "deploy"}

	log.Printf("Generating %d pipeline runs for %s...\n", *count, *repo)

	for i := 0; i < *count; i++ {
		// Random trigger type
		isPush := rand.Float32() > 0.3 // 70% push, 30% PR

		var triggerId int64
		if isPush {
			triggerId, err = createPushTrigger(db, branches)
		} else {
			triggerId, err = createPRTrigger(db, branches)
		}
		if err != nil {
			log.Fatalf("Failed to create trigger: %v", err)
		}

		// Create pipeline
		pipelineRkey := generateRkey()
		sha := generateRandomSha()
		createdTime := time.Now().Add(-time.Duration(rand.Intn(7*24*60)) * time.Minute) // Random time in last week

		_, err = db.Exec(`
			INSERT INTO pipelines (knot, rkey, repo_owner, repo_name, sha, created, trigger_id)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, *knot, pipelineRkey, did, repoName, sha, createdTime.Format(time.RFC3339), triggerId)

		if err != nil {
			log.Fatalf("Failed to create pipeline: %v", err)
		}

		// Create workflow statuses
		numWorkflows := rand.Intn(len(workflows)-1) + 2 // 2-4 workflows
		selectedWorkflows := make([]string, numWorkflows)
		perm := rand.Perm(len(workflows))
		for j := 0; j < numWorkflows; j++ {
			selectedWorkflows[j] = workflows[perm[j]]
		}

		for _, workflow := range selectedWorkflows {
			err = createWorkflowStatuses(db, *knot, pipelineRkey, workflow, createdTime)
			if err != nil {
				log.Fatalf("Failed to create workflow statuses: %v", err)
			}
		}

		log.Printf("Created pipeline %d/%d (rkey: %s)\n", i+1, *count, pipelineRkey)

		// Small delay to ensure unique rkeys
		time.Sleep(2 * time.Millisecond)
	}

	log.Println("✓ Pipeline population complete!")
}

func parseRepo(repo string) (syntax.DID, string, bool) {
	// Simple parser for "did:plc:xyz/reponame"
	for i := 0; i < len(repo); i++ {
		if repo[i] == '/' {
			did := syntax.DID(repo[:i])
			name := repo[i+1:]
			if did != "" && name != "" {
				return did, name, true
			}
		}
	}
	return "", "", false
}

func createPushTrigger(db *sql.DB, branches []string) (int64, error) {
	branch := branches[rand.Intn(len(branches))]
	oldSha := generateRandomSha()
	newSha := generateRandomSha()

	result, err := db.Exec(`
		INSERT INTO triggers (kind, push_ref, push_new_sha, push_old_sha)
		VALUES (?, ?, ?, ?)
	`, "push", "refs/heads/"+branch, newSha, oldSha)

	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func createPRTrigger(db *sql.DB, branches []string) (int64, error) {
	targetBranch := branches[0] // Usually main
	sourceBranch := branches[rand.Intn(len(branches)-1)+1]
	sourceSha := generateRandomSha()
	actions := []string{"opened", "synchronize", "reopened"}
	action := actions[rand.Intn(len(actions))]

	result, err := db.Exec(`
		INSERT INTO triggers (kind, pr_source_branch, pr_target_branch, pr_source_sha, pr_action)
		VALUES (?, ?, ?, ?, ?)
	`, "pull_request", sourceBranch, targetBranch, sourceSha, action)

	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func createWorkflowStatuses(db *sql.DB, knot, pipelineRkey, workflow string, startTime time.Time) error {
	// Generate a progression of statuses for the workflow
	statusProgression := []StatusKind{StatusKindPending, StatusKindRunning}

	// Randomly choose a final status (80% success, 10% failed, 5% timeout, 5% cancelled)
	roll := rand.Float32()
	var finalStatus StatusKind
	switch {
	case roll < 0.80:
		finalStatus = StatusKindSuccess
	case roll < 0.90:
		finalStatus = StatusKindFailed
	case roll < 0.95:
		finalStatus = StatusKindTimeout
	default:
		finalStatus = StatusKindCancelled
	}

	statusProgression = append(statusProgression, finalStatus)

	currentTime := startTime
	for i, status := range statusProgression {
		rkey := fmt.Sprintf("%s-%s-%d", pipelineRkey, workflow, i)

		// Add some realistic time progression (10-60 seconds between statuses)
		if i > 0 {
			currentTime = currentTime.Add(time.Duration(rand.Intn(50)+10) * time.Second)
		}

		var errorMsg *string
		var exitCode int

		if status == StatusKindFailed {
			msg := "Command exited with non-zero status"
			errorMsg = &msg
			exitCode = rand.Intn(100) + 1
		} else if status == StatusKindTimeout {
			msg := "Workflow exceeded maximum execution time"
			errorMsg = &msg
			exitCode = 124
		}

		_, err := db.Exec(`
			INSERT INTO pipeline_statuses (
				spindle, rkey, pipeline_knot, pipeline_rkey,
				created, workflow, status, error, exit_code
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, "spindle.example.com", rkey, knot, pipelineRkey,
			currentTime.Format(time.RFC3339), workflow, string(status), errorMsg, exitCode)

		if err != nil {
			return err
		}
	}

	return nil
}

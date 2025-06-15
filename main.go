// Package main provides a command-line tool for monitoring GitHub repository commits
package main

// Required imports for the application
import (
	"bytes"
	"encoding/json" // For JSON encoding/decoding operations
	"flag"          // For parsing command-line flags
	"fmt"           // For formatted I/O operations
	"io"            // For basic I/O interfaces
	"net/http"      // For making HTTP requests to GitHub
	"os"            // For file and system operations
	"regexp"        // For parsing repository URLs
	"strings"       // For string manipulation operations
	"text/template"
	"time"
)

// Repository represents a monitored GitHub repository
type Repository struct {
	URL        string `json:"url"`         // The GitHub repository URL
	Owner      string `json:"owner"`       // Repository owner/organization
	Repo       string `json:"repo"`        // Repository name
	Branch     string `json:"branch"`      // Branch to monitor (default: main)
	LastCommit string `json:"last_commit"` // Last known commit SHA
	Note       string `json:"note"`        // Optional note explaining what is being monitored
}

// Config represents the application's configuration
type Config struct {
	Repositories []Repository `json:"repositories"`
	Discord      struct {
		Template  string `json:"template"`
		Username  string `json:"username"`
		AvatarURL string `json:"avatar_url"`
	} `json:"discord"`
}

// configFile is the path to the JSON configuration file
const configFile = "config.json"

// DiscordWebhook represents the structure for Discord webhook messages
type DiscordWebhook struct {
	Content   string `json:"content"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
}

// GitHubCommit represents a commit from GitHub API
type GitHubCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Author struct {
			Name  string    `json:"name"`
			Date  time.Time `json:"date"`
		} `json:"author"`
		Message string `json:"message"`
	} `json:"commit"`
	HTMLURL string `json:"html_url"`
}

// Default Discord webhook template
func getDefaultDiscordConfig() struct {
	Template  string
	Username  string
	AvatarURL string
} {
	return struct {
		Template  string
		Username  string
		AvatarURL string
	}{
		Template: `🚨 **New Commits Detected!**

**Repository**: {{.Owner}}/{{.Repo}}
**Branch**: {{.Branch}}
{{if .Note}}**Note**: {{.Note}}{{end}}
**Repository URL**: {{.URL}}

**New Commits**:
{{.Content}}

*Monitored by CSM - Commit Monitor*`,
		Username:  "CSM Commit Monitor", 
		AvatarURL: "https://i.ibb.co/JH5GnN3/Unknown-8.jpg",
	}
}

// sendDiscordNotification sends a notification about detected commits
func sendDiscordNotification(webhookURL string, repo Repository, commits []GitHubCommit, discordConfig struct {
	Template  string
	Username  string
	AvatarURL string
}) error {
	// If no template is provided, use default
	if discordConfig.Template == "" {
		defaults := getDefaultDiscordConfig()
		discordConfig.Template = defaults.Template
	}

	// Create template data structure
	type TemplateData struct {
		Owner   string
		Repo    string
		Branch  string
		Note    string
		URL     string
		Content string
	}

	// Format commits for display
	var commitStrings []string
	for _, commit := range commits {
		commitStr := fmt.Sprintf("• **%s** by %s\n  %s\n  [View Commit](%s)",
			commit.SHA[:8],
			commit.Commit.Author.Name,
			strings.Split(commit.Commit.Message, "\n")[0], // First line of commit message
			commit.HTMLURL)
		commitStrings = append(commitStrings, commitStr)
	}

	// Prepare template data
	data := TemplateData{
		Owner:   repo.Owner,
		Repo:    repo.Repo,
		Branch:  repo.Branch,
		Note:    repo.Note,
		URL:     repo.URL,
		Content: strings.Join(commitStrings, "\n\n"),
	}

	// Parse and execute template
	tmpl, err := template.New("discord").Parse(discordConfig.Template)
	if err != nil {
		return fmt.Errorf("error parsing template: %v", err)
	}

	var description bytes.Buffer
	if err := tmpl.Execute(&description, data); err != nil {
		return fmt.Errorf("error executing template: %v", err)
	}

	webhook := DiscordWebhook{
		Username:  discordConfig.Username,
		Content:   description.String(),
		AvatarURL: discordConfig.AvatarURL,
	}

	payload, err := json.Marshal(webhook)
	if err != nil {
		return err
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("discord webhook returned status: %d", resp.StatusCode)
	}

	return nil
}

// loadConfig reads and parses the configuration file
// Returns an empty config if the file doesn't exist
func loadConfig() (Config, error) {
	var config Config

	// Read the configuration file
	data, err := os.ReadFile(configFile)
	if err != nil {
		// Return empty config with defaults if file doesn't exist
		if os.IsNotExist(err) {
			defaults := getDefaultDiscordConfig()
			return Config{
				Discord: struct {
					Template  string `json:"template"`
					Username  string `json:"username"`
					AvatarURL string `json:"avatar_url"`
				}{
					Template:  defaults.Template,
					Username:  defaults.Username,
					AvatarURL: defaults.AvatarURL,
				},
			}, nil
		}
		return config, err
	}

	// Parse JSON into Config struct
	err = json.Unmarshal(data, &config)
	if err != nil {
		return config, err
	}

	// Set default values if none exist
	defaults := getDefaultDiscordConfig()
	if config.Discord.Template == "" {
		config.Discord.Template = defaults.Template
	}
	if config.Discord.Username == "" {
		config.Discord.Username = defaults.Username
	}
	if config.Discord.AvatarURL == "" {
		config.Discord.AvatarURL = defaults.AvatarURL
	}

	return config, nil
}

// saveConfig writes the current configuration to disk
// The config is pretty-printed with 2-space indentation
func saveConfig(config Config) error {
	// Convert config to pretty-printed JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Write to file with standard permissions (rw-r--r--)
	return os.WriteFile(configFile, data, 0644)
}

// parseGithubRepoURL extracts owner, repo, and branch from a GitHub repository URL
func parseGithubRepoURL(url string) (string, string, string, error) {
	// Handle different GitHub URL formats
	// https://github.com/owner/repo
	// https://github.com/owner/repo/tree/branch
	
	// Remove trailing slash and normalize
	url = strings.TrimSuffix(url, "/")
	
	// Regex to match GitHub repository URLs
	re := regexp.MustCompile(`github\.com/([^/]+)/([^/]+)(?:/tree/([^/]+))?`)
	matches := re.FindStringSubmatch(url)
	
	if matches == nil {
		return "", "", "", fmt.Errorf("invalid GitHub repository URL format")
	}
	
	owner := matches[1]
	repo := matches[2]
	branch := "main" // default branch
	
	if len(matches) > 3 && matches[3] != "" {
		branch = matches[3]
	}
	
	return owner, repo, branch, nil
}

// fetchLatestCommits retrieves the latest commits from a GitHub repository
func fetchLatestCommits(owner, repo, branch string) ([]GitHubCommit, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits?sha=%s&per_page=10", owner, repo, branch)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error fetching commits: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}
	
	var commits []GitHubCommit
	err = json.Unmarshal(body, &commits)
	if err != nil {
		return nil, fmt.Errorf("error parsing commits: %v", err)
	}
	
	return commits, nil
}

// sendTestMessage sends a test message to Discord
func sendTestMessage(webhookURL string) error {
	webhook := DiscordWebhook{
		Username:  "CSM Test Bot",
		Content:   "🎉 **Hello Boy!**\n\nThis is a test message from your CSM Commit Monitor!\n\n✅ GitHub Actions is working correctly\n✅ Discord webhook is connected\n✅ Your monitoring system is ready!\n\n*Test sent at: " + time.Now().Format("2006-01-02 15:04:05 UTC") + "*",
		AvatarURL: "https://i.ibb.co/JH5GnN3/Unknown-8.jpg",
	}

	payload, err := json.Marshal(webhook)
	if err != nil {
		return err
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("discord webhook returned status: %d", resp.StatusCode)
	}

	return nil
}

// addRepository adds a new GitHub repository to monitor
func addRepository(url string, note string) error {
	// Load existing configuration
	config, err := loadConfig()
	if err != nil {
		return err
	}

	// Check for duplicate URLs
	for _, repo := range config.Repositories {
		if repo.URL == url {
			return fmt.Errorf("repository already being monitored")
		}
	}

	// Parse and validate the GitHub URL
	owner, repo, branch, err := parseGithubRepoURL(url)
	if err != nil {
		return err
	}

	// Fetch latest commits to verify repository exists and get initial commit
	commits, err := fetchLatestCommits(owner, repo, branch)
	if err != nil {
		return err
	}
	
	if len(commits) == 0 {
		return fmt.Errorf("no commits found in repository")
	}

	// Add new repository to configuration
	config.Repositories = append(config.Repositories, Repository{
		URL:        url,
		Owner:      owner,
		Repo:       repo,
		Branch:     branch,
		LastCommit: commits[0].SHA, // Store the latest commit SHA
		Note:       note,
	})

	// Save updated configuration
	return saveConfig(config)
}

// removeRepository removes a GitHub repository from monitoring
func removeRepository(url string) error {
	// Load existing configuration
	config, err := loadConfig()
	if err != nil {
		return err
	}

	// Create new slice excluding the specified URL
	newRepos := []Repository{}
	found := false
	for _, repo := range config.Repositories {
		if repo.URL != url {
			newRepos = append(newRepos, repo)
		} else {
			found = true
		}
	}

	// Return error if URL wasn't found
	if !found {
		return fmt.Errorf("repository not found in monitored list")
	}

	// Update and save configuration
	config.Repositories = newRepos
	return saveConfig(config)
}

// checkForNewCommits checks if there are new commits in a repository
func checkForNewCommits(repo Repository) ([]GitHubCommit, error) {
	// Fetch latest commits
	commits, err := fetchLatestCommits(repo.Owner, repo.Repo, repo.Branch)
	if err != nil {
		return nil, err
	}
	
	if len(commits) == 0 {
		return nil, nil
	}
	
	// Find new commits (commits that came after the last known commit)
	var newCommits []GitHubCommit
	for _, commit := range commits {
		if commit.SHA == repo.LastCommit {
			break // Found the last known commit, stop here
		}
		newCommits = append(newCommits, commit)
	}
	
	return newCommits, nil
}

// checkRepositories checks all monitored repositories for new commits
func checkRepositories(webhookURL string) {
	// Load configuration
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	if len(config.Repositories) == 0 {
		fmt.Println("No repositories to monitor")
		return
	}

	// Check each repository for new commits
	for i, repo := range config.Repositories {
		fmt.Printf("Checking repository: %s/%s\n", repo.Owner, repo.Repo)
		
		newCommits, err := checkForNewCommits(repo)
		if err != nil {
			fmt.Printf("Error checking commits for %s/%s: %v\n", repo.Owner, repo.Repo, err)
			continue
		}

		if len(newCommits) == 0 {
			fmt.Printf("No new commits found for %s/%s\n", repo.Owner, repo.Repo)
		} else {
			fmt.Printf("Found %d new commit(s) for %s/%s\n", len(newCommits), repo.Owner, repo.Repo)

			// Send notification
			if err := sendDiscordNotification(webhookURL, repo, newCommits, struct {
				Template  string
				Username  string
				AvatarURL string
			}{
				Template:  config.Discord.Template,
				Username:  config.Discord.Username,
				AvatarURL: config.Discord.AvatarURL,
			}); err != nil {
				fmt.Printf("Error sending notification: %v\n", err)
			} else {
				fmt.Printf("Notification sent for %s/%s\n", repo.Owner, repo.Repo)
			}

			// Update the last known commit SHA
			config.Repositories[i].LastCommit = newCommits[0].SHA
		}
	}
	
	// Save updated configuration with new commit SHAs
	if err := saveConfig(config); err != nil {
		fmt.Printf("Error saving config: %v\n", err)
	}
}

// main is the entry point of the application
func main() {
	// Define command-line flags
	addFlag := flag.String("add", "", "Add a GitHub repository URL to monitor")
	removeFlag := flag.String("remove", "", "Remove a GitHub repository URL from monitoring")
	webhookFlag := flag.String("webhook", "", "Check for new commits using a Discord webhook URL to receive notifications.")
	noteFlag := flag.String("note", "", "(Optional) Note explaining what is being monitored. Must be used with --add")
	testFlag := flag.Bool("test", false, "Send a test message to Discord webhook")
	version := flag.Bool("version", false, "Show the version of the tool")
	flag.Parse()

	if *version {
		fmt.Println("CSM Commit Monitor v1.0.0")
		os.Exit(0)
	}

	// Handle add repository command
	if *addFlag != "" {
		if err := addRepository(*addFlag, *noteFlag); err != nil {
			fmt.Printf("Error adding repository: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Repository added successfully")
		if *noteFlag != "" {
			fmt.Printf("Note added: %s\n", *noteFlag)
		}
	}

	// Handle remove repository command
	if *removeFlag != "" {
		if err := removeRepository(*removeFlag); err != nil {
			fmt.Printf("Error removing repository: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Repository removed successfully")
	}

	// Handle test command
	if *testFlag {
		if *webhookFlag == "" {
			fmt.Println("Error: --webhook flag is required for testing")
			fmt.Println("Usage: csm --test --webhook <discord_webhook_url>")
			os.Exit(1)
		}
		fmt.Println("Sending test message...")
		if err := sendTestMessage(*webhookFlag); err != nil {
			fmt.Printf("Error sending test message: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Test message sent successfully!")
		return
	}

	// If no add/remove/test flags, require webhook for checking commits
	if *addFlag == "" && *removeFlag == "" {
		if *webhookFlag == "" {
			fmt.Println("Error: --webhook flag is required when checking for commits")
			fmt.Println("Usage: csm --webhook <discord_webhook_url>")
			os.Exit(1)
		}
		// Call checkRepositories with the webhook URL
		checkRepositories(*webhookFlag)
	}
}

// Package main provides a command-line tool for monitoring changes in GitHub code snippets
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
	"regexp"        // For parsing line numbers from URLs
	"strings"       // For string manipulation operations
	"text/template"
)

// CodeSnippet represents a monitored code segment from GitHub
type CodeSnippet struct {
	URL     string   `json:"url"`     // The full GitHub URL of the code snippet
	Company string   `json:"company"` // The name of the company being monitored
	Content []string `json:"content"` // The lines of code being monitored
	Note    string   `json:"note"`    // Optional note explaining what is being monitored
}

// Config represents the application's configuration
type Config struct {
	Snippets []CodeSnippet `json:"snippets"`
	Discord  struct {
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
		Template: `Changes detected!

**Company**: {{.Company}}
{{if .Note}}
**Note**: {{.Note}}
{{end}}
**File**: {{.URL}}

**Original Code**:
` + "```" + `javascript
{{.Content}}
` + "```" + `
`,
		Username:  "csm",
		AvatarURL: "https://i.ibb.co/JH5GnN3/Unknown-8.jpg",
	}
}

// sendDiscordNotification sends a notification about detected changes
func sendDiscordNotification(webhookURL string, snippet CodeSnippet, discordConfig struct {
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
		Company string
		Note    string
		URL     string
		Content string
	}

	// Prepare template data
	data := TemplateData{
		Company: snippet.Company,
		Note:    snippet.Note,
		URL:     snippet.URL,
		Content: strings.Join(snippet.Content, "\n"),
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

// parseGithubURL extracts the raw content URL and line numbers from a GitHub URL
// Returns the raw URL, start line, end line, and any error encountered
func parseGithubURL(url string) (string, int, int, error) {
	// Regular expression to extract line numbers from URL fragment
	re := regexp.MustCompile(`#L(\d+)-L(\d+)`)
	matches := re.FindStringSubmatch(url)
	if matches == nil {
		return "", 0, 0, fmt.Errorf("Invalid URL format: must contain line numbers (e.g., #L52-L64)")
	}

	// Convert GitHub web URL to raw content URL
	rawURL := strings.Replace(url, "github.com", "raw.githubusercontent.com", 1)
	rawURL = strings.Replace(rawURL, "/blob/", "/", 1)
	rawURL = strings.Split(rawURL, "#")[0]

	// Parse line numbers from regex matches
	startLine, endLine := 0, 0
	fmt.Sscanf(matches[1], "%d", &startLine)
	fmt.Sscanf(matches[2], "%d", &endLine)

	return rawURL, startLine, endLine, nil
}

// fetchCodeContent retrieves the specified lines from a raw GitHub URL
// Returns the lines of code between startLine and endLine (inclusive)
func fetchCodeContent(rawURL string, startLine, endLine int) ([]string, error) {
	// Fetch file content from GitHub
	resp, err := http.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read the entire response body
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Split content into lines and validate line numbers
	lines := strings.Split(string(content), "\n")
	if startLine > len(lines) || endLine > len(lines) || startLine < 1 || endLine < 2 {
		return nil, fmt.Errorf("line numbers out of range. start line must be > 0 and end line must be > 1")
	}

	// Extract and return the requested lines
	return lines[startLine-1 : endLine], nil
}

// addURL adds a new GitHub URL to the monitored snippets
// It fetches the current content and saves it to the configuration
func addURL(url string, note string) error {
	// Load existing configuration
	config, err := loadConfig()
	if err != nil {
		return err
	}

	// Check for duplicate URLs
	for _, snippet := range config.Snippets {
		if snippet.URL == url {
			return fmt.Errorf("URL already being monitored")
		}
	}

	// Parse and validate the GitHub URL
	rawURL, startLine, endLine, err := parseGithubURL(url)
	if err != nil {
		return err
	}

	// Fetch the initial content
	content, err := fetchCodeContent(rawURL, startLine, endLine)
	if err != nil {
		return err
	}

	company, err := extractCompanyName(url)
	if err != nil {
		return err
	}

	// Add new snippet to configuration
	config.Snippets = append(config.Snippets, CodeSnippet{
		URL:     url,
		Company: company,
		Content: content,
		Note:    note,
	})

	// Save updated configuration
	return saveConfig(config)
}

func extractCompanyName(githubURL string) (string, error) {
	name := strings.Split(strings.Split(githubURL, "github.com/")[1], "/")[0]
	return name, nil
}

// removeURL removes a GitHub URL from the monitored snippets
func removeURL(url string) error {
	// Load existing configuration
	config, err := loadConfig()
	if err != nil {
		return err
	}

	// Create new slice excluding the specified URL
	newSnippets := []CodeSnippet{}
	found := false
	for _, snippet := range config.Snippets {
		if snippet.URL != url {
			newSnippets = append(newSnippets, snippet)
		} else {
			found = true
		}
	}

	// Return error if URL wasn't found
	if !found {
		return fmt.Errorf("URL not found in monitored list")
	}

	// Update and save configuration
	config.Snippets = newSnippets
	return saveConfig(config)
}

// checkCodeExistence verifies if a saved code snippet still exists in the current file content
// Returns true if the snippet is found, false otherwise
func checkCodeExistence(snippet CodeSnippet) (bool, error) {
	// Get raw URL for content fetching
	rawURL, _, _, err := parseGithubURL(snippet.URL)
	if err != nil {
		return false, fmt.Errorf("error parsing URL: %v", err)
	}

	// Fetch current content from GitHub
	resp, err := http.Get(rawURL)
	if err != nil {
		return false, fmt.Errorf("error fetching content: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("error reading content: %v", err)
	}

	// Prepare saved snippet and current content for comparison
	savedSnippet := strings.Join(snippet.Content, "\n")
	currentContent := string(content)

	// Normalize line endings
	savedSnippet = strings.ReplaceAll(savedSnippet, "\r\n", "\n")
	currentContent = strings.ReplaceAll(currentContent, "\r\n", "\n")

	// Normalize whitespace in saved snippet
	savedLines := strings.Split(savedSnippet, "\n")
	for i, line := range savedLines {
		savedLines[i] = strings.TrimSpace(line)
	}
	savedSnippet = strings.Join(savedLines, "\n")

	// Normalize whitespace in current content
	currentLines := strings.Split(currentContent, "\n")
	for i, line := range currentLines {
		currentLines[i] = strings.TrimSpace(line)
	}
	currentContent = strings.Join(currentLines, "\n")

	// Check if normalized snippet exists in normalized content
	return strings.Contains(currentContent, savedSnippet), nil
}

// checkChanges checks all monitored snippets for changes
func checkChanges(webhookURL string) {
	// Load configuration
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	if len(config.Snippets) == 0 {
		fmt.Println("No snippets to monitor")
		return
	}

	// Check each snippet for changes
	for _, snippet := range config.Snippets {
		unchanged, err := checkCodeExistence(snippet)
		if err != nil {
			fmt.Printf("Error fetching content for %s: %v\n", snippet.URL, err)
			continue
		}

		if unchanged {
			fmt.Printf("No changes detected for %s\n", snippet.URL)
		} else {
			fmt.Printf("Changes detected for %s\n", snippet.URL)

			// Send notification using the settings from config
			if err := sendDiscordNotification(webhookURL, snippet, struct {
				Template  string
				Username  string
				AvatarURL string
			}{
				Template:  config.Discord.Template,
				Username:  config.Discord.Username,
				AvatarURL: config.Discord.AvatarURL,
			}); err != nil {
				fmt.Printf("Error sending notification: %v\n", err)
			}

			removeURL(snippet.URL)
		}
	}
}

// main is the entry point of the application
func main() {
	// Define command-line flags
	addFlag := flag.String("add", "", "Add a GitHub URL to monitor")
	removeFlag := flag.String("remove", "", "Remove a GitHub URL from monitoring")
	webhookFlag := flag.String("webhook", "", "Check for code changes using a Discord webhook URL to receive notifications.")
	noteFlag := flag.String("note", "", "(Optional) Note explaining what is being monitored. Must be used with --add")
	version := flag.Bool("version", false, "Show the version of the tool")
	flag.Parse()

	if *version {
		fmt.Println("0.1.0")
		os.Exit(0)
	}

	// Handle add URL command
	if *addFlag != "" {
		if err := addURL(*addFlag, *noteFlag); err != nil {
			fmt.Printf("Error adding URL: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("URL added successfully")
		if *noteFlag != "" {
			fmt.Printf("Note added: %s\n", *noteFlag)
		}
	}

	// Handle remove URL command
	if *removeFlag != "" {
		if err := removeURL(*removeFlag); err != nil {
			fmt.Printf("Error removing URL: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("URL removed successfully")
	}

	// If no add/remove flags, require webhook for checking changes
	if *addFlag == "" && *removeFlag == "" {
		if *webhookFlag == "" {
			fmt.Println("Error: --webhook flag is required when checking for changes")
			fmt.Println("Usage: tool --webhook <discord_webhook_url>")
			os.Exit(1)
		}
		// Call checkChanges with the webhook URL
		checkChanges(*webhookFlag)
	}
}

package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "strings"
    "sync"

    gowiki "github.com/trietmn/go-wiki"
    openai "github.com/openai/openai-go"
    "github.com/openai/openai-go/option"
    "gopkg.in/yaml.v2"
    "maunium.net/go/mautrix"
    "maunium.net/go/mautrix/event"
    "maunium.net/go/mautrix/id"
)

const (
    configFileName = "config.yaml"
    summaryFileExt  = ".txt"
    filePerm        = 0644
)

// Config holds the configuration settings for the application.
type Config struct {
    Matrix struct {
        Homeserver  string `yaml:"homeserver"`
        UserID      string `yaml:"user_id"`
        AccessToken string `yaml:"access_token"`
    } `yaml:"matrix"`
    OpenAI struct {
        APIKey       string `yaml:"api_key"`
        Model        string `yaml:"model"`
        SystemPrompt string `yaml:"system_prompt"`
    } `yaml:"openai"`
    Wikipedia struct {
        Prefix string `yaml:"prefix"`
    } `yaml:"wikipedia"`
    Bot struct {
        OutputDir string `yaml:"output_dir"`
        Command   string `yaml:"command"`
    } `yaml:"bot"`
}

var (
    // Global variables for configuration and clients
    config       Config
    openaiClient *openai.Client
    matrixClient *mautrix.Client
    outputDir    string
    wg           sync.WaitGroup
)

func main() {
    // Load configuration from file
    err := loadConfig(configFileName)
    if err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }

    // Ensure output directory exists
    outputDir = config.Bot.OutputDir
    if err := ensureDirExists(outputDir); err != nil {
        log.Fatalf("Failed to create output directory: %v", err)
    }

    // Initialize OpenAI client with API key
    openaiClient = openai.NewClient(
        option.WithAPIKey(config.OpenAI.APIKey),
    )

    // Initialize Matrix client for communication
    matrixClient, err = mautrix.NewClient(config.Matrix.Homeserver, id.UserID(config.Matrix.UserID), config.Matrix.AccessToken)
    if err != nil {
        log.Fatalf("Failed to create Matrix client: %v", err)
    }

    // Set up event handler for message events
    syncer := matrixClient.Syncer.(*mautrix.DefaultSyncer)
    syncer.OnEventType(event.EventMessage, handleMessageEvent)

    // Start the Matrix client sync process
    if err = matrixClient.Sync(); err != nil {
        log.Fatalf("Matrix sync failed: %v", err)
    }

    // Wait for all goroutines to finish
    wg.Wait()
}

// loadConfig reads the configuration from a YAML file.
func loadConfig(filename string) error {
    data, err := os.ReadFile(filename)
    if err != nil {
        return err
    }
    err = yaml.Unmarshal(data, &config)
    if err != nil {
        return err
    }
    return nil
}

// ensureDirExists creates a directory if it does not exist.
func ensureDirExists(dir string) error {
    if _, err := os.Stat(dir); os.IsNotExist(err) {
        return os.MkdirAll(dir, os.ModePerm)
    }
    return nil
}

// handleMessageEvent processes incoming message events.
func handleMessageEvent(ctx context.Context, ev *event.Event) {
    // Ignore messages sent by the bot itself
    if ev.Sender == id.UserID(config.Matrix.UserID) {
        return
    }

    msgEvent := ev.Content.AsMessage()
    if msgEvent == nil {
        return
    }

    // Check if the message starts with the bot command
    if strings.HasPrefix(msgEvent.Body, config.Bot.Command+" ") {
        searchTerm := strings.TrimSpace(strings.TrimPrefix(msgEvent.Body, config.Bot.Command+" "))
        wg.Add(1)
        go func() {
            defer wg.Done()
            handleWikiCommand(ctx, ev.RoomID, searchTerm)
        }()
    }
}

// handleWikiCommand searches Wikipedia and sends a summary to the chat.
func handleWikiCommand(ctx context.Context, roomID id.RoomID, searchTerm string) {
    articleTitle, err := searchWikipedia(searchTerm)
    if err != nil {
        sendMessage(ctx, roomID, fmt.Sprintf("Error finding article: %v", err))
        return
    }

    summaryFile := fmt.Sprintf("%s/%s%s", outputDir, sanitizeFileName(articleTitle), summaryFileExt)
    if summary, err := readSummaryFromFile(summaryFile); err == nil {
        sendMessage(ctx, roomID, summary)
        return
    }

    page, err := gowiki.GetPage(articleTitle, -1, false, true)
    if err != nil {
        sendMessage(ctx, roomID, fmt.Sprintf("Error fetching article: %v", err))
        return
    }

    content, err := page.GetContent()
    if err != nil {
        sendMessage(ctx, roomID, fmt.Sprintf("Error getting content: %v", err))
        return
    }

    summary, err := summarizeContent(ctx, content)
    if err != nil {
        sendMessage(ctx, roomID, fmt.Sprintf("Error summarising article: %v", err))
        return
    }

    if err = writeSummaryToFile(summaryFile, summary); err != nil {
        sendMessage(ctx, roomID, fmt.Sprintf("Error saving summary: %v", err))
        return
    }

    sendMessage(ctx, roomID, summary)
}

// readSummaryFromFile reads a summary from a file if it exists.
func readSummaryFromFile(filename string) (string, error) {
    if _, err := os.Stat(filename); err == nil {
        summary, err := os.ReadFile(filename)
        if err != nil {
            return "", fmt.Errorf("error reading summary: %v", err)
        }
        return string(summary), nil
    }
    return "", fmt.Errorf("summary file does not exist")
}

// writeSummaryToFile writes a summary to a file.
func writeSummaryToFile(filename, summary string) error {
    return os.WriteFile(filename, []byte(summary), filePerm)
}

// searchWikipedia searches for a Wikipedia article by term.
func searchWikipedia(searchTerm string) (string, error) {
    searchResults, _, err := gowiki.Search(searchTerm, 1, false)
    if err != nil {
        return "", err
    }
    if len(searchResults) == 0 {
        return "", fmt.Errorf("No results found for '%s'", searchTerm)
    }
    return searchResults[0], nil
}

// summarizeContent uses OpenAI to summarise the Wikipedia content.
func summarizeContent(ctx context.Context, content string) (string, error) {
    req := openai.ChatCompletionNewParams{
        Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
            openai.SystemMessage(config.OpenAI.SystemPrompt),
            openai.UserMessage(content),
        }),
        Model: openai.F(config.OpenAI.Model),
    }

    resp, err := openaiClient.Chat.Completions.New(ctx, req)
    if err != nil {
        return "", err
    }

    if len(resp.Choices) > 0 {
        return resp.Choices[0].Message.Content, nil
    }

    return "", fmt.Errorf("no response from OpenAI")
}

// sendMessage sends a text message to a Matrix room.
func sendMessage(ctx context.Context, roomID id.RoomID, message string) {
    _, err := matrixClient.SendText(ctx, roomID, message)
    if err != nil {
        log.Printf("Failed to send message to %s: %v", roomID, err)
    }
}

// sanitizeFileName replaces invalid filename characters with underscores.
func sanitizeFileName(name string) string {
    invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
    for _, char := range invalidChars {
        name = strings.ReplaceAll(name, char, "_")
    }
    return name
}

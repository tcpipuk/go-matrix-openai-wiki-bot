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
    "gopkg.in/yaml.v2"
    "maunium.net/go/mautrix"
    "maunium.net/go/mautrix/event"
    "maunium.net/go/mautrix/id"
)

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
    config       Config
    openaiClient *openai.Client
    matrixClient *mautrix.Client
    outputDir    string
    wg           sync.WaitGroup
)

func main() {
    // Load configuration
    err := loadConfig("config.yaml")
    if err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }

    // Ensure output directory exists
    outputDir = config.Bot.OutputDir
    if _, err := os.Stat(outputDir); os.IsNotExist(err) {
        os.MkdirAll(outputDir, os.ModePerm)
    }

    // Initialize OpenAI client
    openaiClient = openai.NewClient(config.OpenAI.APIKey)

    // Initialize Matrix client
    matrixClient, err = mautrix.NewClient(config.Matrix.Homeserver, id.UserID(config.Matrix.UserID), config.Matrix.AccessToken)
    if err != nil {
        log.Fatalf("Failed to create Matrix client: %v", err)
    }

    // Sync the Matrix client
    syncer := matrixClient.Syncer.(*mautrix.DefaultSyncer)
    syncer.OnEventType(event.EventMessage, mautrix.EventHandlerFunc(handleMessageEvent))

    // Start syncing
    err = matrixClient.Sync()
    if err != nil {
        log.Fatalf("Matrix sync failed: %v", err)
    }

    // Wait indefinitely
    select {}
}

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

func handleMessageEvent(ev *event.Event) {
    if ev.Sender == id.UserID(config.Matrix.UserID) {
        return
    }

    msgEvent, ok := ev.Content.AsMessage()
    if !ok {
        return
    }

    if strings.HasPrefix(msgEvent.Body, config.Bot.Command+" ") {
        searchTerm := strings.TrimSpace(strings.TrimPrefix(msgEvent.Body, config.Bot.Command+" "))
        wg.Add(1)
        go func() {
            defer wg.Done()
            handleWikiCommand(ev.RoomID, searchTerm)
        }()
    }
}

func handleWikiCommand(roomID id.RoomID, searchTerm string) {
    articleTitle, err := searchWikipedia(searchTerm)
    if err != nil {
        sendMessage(roomID, fmt.Sprintf("Error finding article: %v", err))
        return
    }

    summaryFile := fmt.Sprintf("%s/%s.txt", outputDir, sanitizeFileName(articleTitle))
    if _, err := os.Stat(summaryFile); err == nil {
        summary, err := os.ReadFile(summaryFile)
        if err != nil {
            sendMessage(roomID, fmt.Sprintf("Error reading summary: %v", err))
            return
        }
        sendMessage(roomID, string(summary))
        return
    }

    page, err := gowiki.GetPage(articleTitle, -1, false, true)
    if err != nil {
        sendMessage(roomID, fmt.Sprintf("Error fetching article: %v", err))
        return
    }

    content, err := page.GetContent()
    if err != nil {
        sendMessage(roomID, fmt.Sprintf("Error getting content: %v", err))
        return
    }

    summary, err := summarizeContent(content)
    if err != nil {
        sendMessage(roomID, fmt.Sprintf("Error summarizing article: %v", err))
        return
    }

    err = os.WriteFile(summaryFile, []byte(summary), 0644)
    if err != nil {
        sendMessage(roomID, fmt.Sprintf("Error saving summary: %v", err))
        return
    }

    sendMessage(roomID, summary)
}

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

func summarizeContent(content string) (string, error) {
    ctx := context.Background()

    req := openai.ChatCompletionRequest{
        Model: config.OpenAI.Model,
        Messages: []openai.ChatCompletionMessage{
            {Role: openai.ChatMessageRoleSystem, Content: config.OpenAI.SystemPrompt},
            {Role: openai.ChatMessageRoleUser, Content: content},
        },
    }

    resp, err := openaiClient.CreateChatCompletion(ctx, req)
    if err != nil {
        return "", err
    }

    if len(resp.Choices) > 0 {
        return resp.Choices[0].Message.Content, nil
    }

    return "", fmt.Errorf("no response from OpenAI")
}

func sendMessage(roomID id.RoomID, message string) {
    _, err := matrixClient.SendText(roomID, message)
    if err != nil {
        log.Printf("Failed to send message to %s: %v", roomID, err)
    }
}

func sanitizeFileName(name string) string {
    invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
    for _, char := range invalidChars {
        name = strings.ReplaceAll(name, char, "_")
    }
    return name
}

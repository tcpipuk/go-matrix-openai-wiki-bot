package main

import (
    "context"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "path/filepath"
    "strings"
    "sync"

    gowiki "github.com/trietmn/go-wiki"
    openai "github.com/openai/openai-go"
    "gopkg.in/yaml.v2"
    "maunium.net/go/mautrix"
    "maunium.net/go/mautrix/event"
    "maunium.net/go/mautrix/id"
)

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
    syncer.OnEventType(event.EventMessage, func(ev *event.Event) {
        // Ignore messages from the bot itself
        if ev.Sender == id.UserID(config.Matrix.UserID) {
            return
        }

        // Handle the configured command
        if msgEvent, ok := ev.Content.AsMessage(); ok {
            if strings.HasPrefix(msgEvent.Body, config.Bot.Command+" ") {
                searchTerm := strings.TrimSpace(strings.TrimPrefix(msgEvent.Body, config.Bot.Command+" "))
                wg.Add(1)
                go func() {
                    defer wg.Done()
                    handleWikiCommand(ev.RoomID, searchTerm)
                }()
            }
        }
    })

    // Start syncing in a separate Goroutine
    go func() {
        err = matrixClient.Sync()
        if err != nil {
            log.Fatalf("Matrix sync failed: %v", err)
        }
    }()

    // Wait indefinitely
    select {}
}

func summarizeContent(content string) (string, error) {
    ctx := context.Background()
    
    req := openai.ChatCompletionRequest{
        Model:       "gpt-4",
        Messages: []openai.ChatCompletionMessage{
            {Role: openai.ChatMessageRoleSystem, Content: config.OpenAI.SystemPrompt},
            {Role: openai.ChatMessageRoleUser, Content: content},
        },
    }

    resp, err := openaiClient.ChatCompletions.Create(ctx, req)
    if err != nil {
        return "", err
    }

    if len(resp.Choices) > 0 {
        return resp.Choices[0].Message.Content, nil
    }

    return "", fmt.Errorf("no response from OpenAI")
}

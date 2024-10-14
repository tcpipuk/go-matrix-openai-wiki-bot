# go-matrix-openai-wiki-bot

A Matrix bot that searches Wikipedia for articles based on user commands, summarizes them using OpenAI's GPT model, caches the summaries, and replies in Matrix rooms.

## Features

- **Wikipedia Search**: Searches for articles on Wikipedia based on user input.
- **Article Summarization**: Uses OpenAI's GPT model to summarize articles.
- **Caching Mechanism**: Caches summaries to avoid redundant API calls.
- **Configurable**: All settings are configurable via a YAML file.
- **Concurrent Requests**: Handles multiple requests concurrently using Goroutines.

## Prerequisites

- Go (version 1.18 or higher)
- Docker (if you prefer running the bot in a container)
- Git (for cloning the repository)
- An OpenAI API key
- A Matrix account and access token

## Setup Instructions

### **1. Clone the Repository**

```bash
git clone https://github.com/tcpipuk/go-matrix-openai-wiki-bot.git
cd go-matrix-openai-wiki-bot
```

### **2. Update Configuration**

- Copy the sample configuration file:

  ```bash
  cp config.yaml.example config.yaml
  ```

- Edit `config.yaml` and update the following fields:

  - **Matrix Configuration**:
    - `homeserver`: Your Matrix homeserver URL.
    - `user_id`: The Matrix user ID of your bot.
    - `access_token`: The access token for your bot.

  - **OpenAI Configuration**:
    - `api_key`: Your OpenAI API key.
    - `model`: The OpenAI model you wish to use (e.g., `gpt-3.5-turbo`).
    - `system_prompt`: The prompt provided to the model.

  - **Bot Configuration**:
    - `output_dir`: Directory where summaries will be cached.
    - `command`: The command prefix to trigger the bot (e.g., `!wiki`).

### **3. Build and Run the Bot**

#### **Option 1: Run Directly**

Ensure you have Go installed.

- Install dependencies:

  ```bash
  go mod download
  ```

- Build the application:

  ```bash
  go build -o go-matrix-openai-wiki-bot .
  ```

- Run the application:

  ```bash
  ./go-matrix-openai-wiki-bot
  ```

#### **Option 2: Run with Docker**

- Build the Docker image:

  ```bash
  docker build -t go-matrix-openai-wiki-bot .
  ```

- Run the Docker container:

  ```bash
  docker run -d \
    -v $(pwd)/config.yaml:/app/config.yaml \
    -v $(pwd)/output:/app/output \
    --name go-matrix-openai-wiki-bot \
    go-matrix-openai-wiki-bot
  ```

**Note:** The `-v` flags mount your local `config.yaml` and `output` directory into the container.

### **4. Test the Bot**

- Invite your bot to a Matrix room.
- Send a message using the configured command, e.g.:

  ```
  !wiki Computer
  ```

- The bot should reply with a summarized version of the Wikipedia article.

## Docker Image from GitHub Container Registry

Alternatively, you can pull the pre-built Docker image from GHCR:

```bash
docker pull ghcr.io/your_username/go-matrix-openai-wiki-bot:latest
```

Replace `your_username` with your GitHub username.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

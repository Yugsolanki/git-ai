package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

const promptTemplate = `Generate a git commit message in Conventional Commits format based on the diff below.

STRICT FORMAT:
<type>(<scope>): <subject>
- <bullet point 1>

CRITICAL CONSTRAINTS:
• Output ONLY the raw message text.
• Types: feat, fix, docs, style, refactor, test, chore, perf, ci, build, revert
• Subject: imperative mood, lowercase start, no period
• No explanations, no quotes, no markdown, no self "this commit" references, 
• No backticks, no emojis, no tags, no history references.

Diff:
%s`

const tagPromptTemplate = `Generate a concise, high-level release summary based on the following commit history.

CRITICAL CONSTRAINTS:
- Output ONLY the raw summary text.
- Do NOT include any introductory or meta-text.
- Group by feature or fix if multiple changes exist, but keep it brief.
- Do NOT mention "History" or "Tag" in the response.
- No backticks, no emojis, no tags, no quotes, no markdown, no self "this tag" references, 

COMMIT HISTORY:
%s`

// OpenRouter supports many models; pick one that fits your needs
const openRouterModel = "qwen/qwen3-235b-a22b-2507"

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		log.Fatal("Please set the OPENROUTER_API_KEY environment variable")
	}

	// Configure OpenAI client to use OpenRouter
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://openrouter.ai/api/v1"

	client := openai.NewClientWithConfig(config)

	// Determine if we are doing a commit or a tag
	arg := ""
	if len(os.Args) > 1 {
		arg = os.Args[1]
	}

	if arg == "tag" {
		generateTagMessage(ctx, client)
	} else {
		generateCommitMessage(ctx, client)
	}
}

func generateCommitMessage(ctx context.Context, client *openai.Client) {
	// 1. Get Staged Changes
	diff, err := exec.Command("git", "diff", "--cached").Output()
	if err != nil {
		log.Printf("Error getting git diff: %v", err)
		return
	}
	if len(diff) == 0 {
		fmt.Println("No staged changes found. Use 'git add' first.")
		return
	}

	// 2. Generate AI Message
	prompt := fmt.Sprintf(promptTemplate, string(diff))
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openRouterModel,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		MaxTokens: 200, // Keep commit messages concise
	})
	if err != nil {
		log.Fatal(err)
	}

	message := formatAIResponse(resp)
	fmt.Printf("\nProposed Commit Message:\n\033[32m%s\033[0m\n\n", message)

	// 3. Optional: Execute the commit
	fmt.Print("Apply this commit? (y/n): ")
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(strings.TrimSpace(confirm)) == "y" {
		out, err := exec.Command("git", "commit", "-m", message).CombinedOutput()
		if err != nil {
			log.Printf("Commit failed: %v\n%s", err, string(out))
		} else {
			fmt.Println(string(out))
		}
	}
}

func generateTagMessage(ctx context.Context, client *openai.Client) {
	// Get the last tag safely
	lastTag, err := exec.Command("git", "describe", "--tags", "--abbrev=0").Output()
	if err != nil {
		// No tags yet? Use initial commit
		logs, err := exec.Command("git", "log", "--oneline").Output()
		if err != nil {
			log.Printf("Error getting git log: %v", err)
			return
		}
		prompt := fmt.Sprintf(tagPromptTemplate, string(logs))
		sendTagPrompt(ctx, client, prompt)
		return
	}

	// Get logs since last tag
	logs, err := exec.Command("git", "log", strings.TrimSpace(string(lastTag))+"..HEAD", "--oneline").Output()
	if err != nil {
		log.Printf("Error getting git log: %v", err)
		return
	}

	prompt := fmt.Sprintf(tagPromptTemplate, string(logs))
	sendTagPrompt(ctx, client, prompt)
}

func sendTagPrompt(ctx context.Context, client *openai.Client, prompt string) {
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openRouterModel,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		MaxTokens: 300,
	})
	if err != nil {
		log.Fatal(err)
	}

	message := formatAIResponse(resp)
	fmt.Printf("\nProposed Tag Message:\n\033[36m%s\033[0m\n", message)
}

func formatAIResponse(resp openai.ChatCompletionResponse) string {
	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return "⚠️ No response generated"
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content)
}

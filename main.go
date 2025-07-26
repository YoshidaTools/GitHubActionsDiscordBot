package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/bwmarrin/discordgo"
	"github.com/google/go-github/v56/github"
)

type Bot struct {
	discord   *discordgo.Session
	github    *github.Client
	guildID   string
	channelID string
}

func main() {
	bot, err := NewBot()
	if err != nil {
		log.Fatal("Error creating bot:", err)
	}

	bot.Start()
}

func NewBot() (*Bot, error) {
	// Discord session
	dg, err := discordgo.New("Bot " + os.Getenv("DISCORD_TOKEN"))
	if err != nil {
		return nil, err
	}

	// GitHub App authentication
	appID, _ := strconv.ParseInt(os.Getenv("GITHUB_APP_ID"), 10, 64)
	installationID, _ := strconv.ParseInt(os.Getenv("GITHUB_INSTALLATION_ID"), 10, 64)

	itr, err := ghinstallation.NewKeyFromFile(
		http.DefaultTransport,
		appID,
		installationID,
		os.Getenv("GITHUB_PRIVATE_KEY_PATH"),
	)
	if err != nil {
		return nil, err
	}

	ghClient := github.NewClient(&http.Client{Transport: itr})

	return &Bot{
		discord:   dg,
		github:    ghClient,
		guildID:   os.Getenv("DISCORD_GUILD_ID"),
		channelID: os.Getenv("DISCORD_CHANNEL_ID"),
	}, nil
}

func (b *Bot) Start() {
	b.discord.AddHandler(b.messageCreate)
	b.discord.AddHandler(b.interactionCreate)
	b.discord.AddHandler(b.ready)

	err := b.discord.Open()
	if err != nil {
		log.Fatal("Error opening connection:", err)
	}
	defer b.discord.Close()

	b.registerSlashCommands()

	log.Println("Bot is running...")
	select {} // Keep running
}

func (b *Bot) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	content := strings.TrimSpace(m.Content)
	if !strings.HasPrefix(content, "!gh") {
		return
	}

	args := strings.Fields(content)[1:]
	if len(args) == 0 {
		b.sendHelp(s, m.ChannelID)
		return
	}

	switch args[0] {
	case "workflows":
		b.handleListWorkflows(s, m, args[1:])
	case "run":
		b.handleRunWorkflow(s, m, args[1:])
	case "status":
		b.handleWorkflowStatus(s, m, args[1:])
	case "logs":
		b.handleWorkflowLogs(s, m, args[1:])
	default:
		b.sendHelp(s, m.ChannelID)
	}
}

func (b *Bot) handleListWorkflows(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 2 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `!gh workflows <owner> <repo>`")
		return
	}

	owner, repo := args[0], args[1]

	workflows, _, err := b.github.Actions.ListWorkflows(
		context.Background(), owner, repo, nil,
	)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Error: %v", err))
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("Workflows in %s/%s", owner, repo),
		Color: 0x00ff00,
	}

	for _, workflow := range workflows.Workflows {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   *workflow.Name,
			Value:  fmt.Sprintf("ID: %d\nState: %s", *workflow.ID, *workflow.State),
			Inline: true,
		})
	}

	s.ChannelMessageSendEmbed(m.ChannelID, embed)
}

func (b *Bot) handleRunWorkflow(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 3 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `!gh run <owner> <repo> <workflow_id> [ref]`")
		return
	}

	owner, repo := args[0], args[1]
	workflowID, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "Invalid workflow ID")
		return
	}

	ref := "main"
	if len(args) > 3 {
		ref = args[3]
	}

	event := github.CreateWorkflowDispatchEventRequest{
		Ref: ref,
	}

	_, err = b.github.Actions.CreateWorkflowDispatchEventByID(
		context.Background(), owner, repo, workflowID, event,
	)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Error running workflow: %v", err))
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "Workflow Started",
		Description: fmt.Sprintf("Workflow %d in %s/%s has been triggered", workflowID, owner, repo),
		Color:       0x00ff00,
	}

	s.ChannelMessageSendEmbed(m.ChannelID, embed)
}

func (b *Bot) handleWorkflowStatus(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 2 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `!gh status <owner> <repo> [workflow_id]`")
		return
	}

	owner, repo := args[0], args[1]

	var runs *github.WorkflowRuns
	var err error

	if len(args) > 2 {
		workflowID, parseErr := strconv.ParseInt(args[2], 10, 64)
		if parseErr != nil {
			s.ChannelMessageSend(m.ChannelID, "Invalid workflow ID")
			return
		}
		runs, _, err = b.github.Actions.ListWorkflowRunsByID(
			context.Background(), owner, repo, workflowID, &github.ListWorkflowRunsOptions{},
		)
	} else {
		runs, _, err = b.github.Actions.ListRepositoryWorkflowRuns(
			context.Background(), owner, repo, &github.ListWorkflowRunsOptions{},
		)
	}

	if err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Error: %v", err))
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("Recent Workflow Runs - %s/%s", owner, repo),
		Color: 0x0099ff,
	}

	for i, run := range runs.WorkflowRuns {
		if i >= 5 { // Limit to 5 recent runs
			break
		}

		status := *run.Status
		color := "🟡"
		if status == "completed" {
			if *run.Conclusion == "success" {
				color = "🟢"
			} else {
				color = "🔴"
			}
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name: fmt.Sprintf("%s %s", color, *run.Name),
			Value: fmt.Sprintf("Status: %s\nRun ID: %d\nCreated: %s",
				status, *run.ID, run.CreatedAt.Format("2006-01-02 15:04:05")),
			Inline: true,
		})
	}

	s.ChannelMessageSendEmbed(m.ChannelID, embed)
}

func (b *Bot) handleWorkflowLogs(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 3 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `!gh logs <owner> <repo> <run_id>`")
		return
	}

	owner, repo := args[0], args[1]
	runID, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "Invalid run ID")
		return
	}

	logs, _, err := b.github.Actions.GetWorkflowRunLogs(
		context.Background(), owner, repo, runID, 1,
	)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Error getting logs: %v", err))
		return
	}
	_ = logs

	embed := &discordgo.MessageEmbed{
		Title:       "Workflow Logs",
		Description: fmt.Sprintf("Logs for run %d in %s/%s", runID, owner, repo),
		Color:       0x0099ff,
	}

	s.ChannelMessageSendEmbed(m.ChannelID, embed)
}

func (b *Bot) sendHelp(s *discordgo.Session, channelID string) {
	embed := &discordgo.MessageEmbed{
		Title: "GitHub Actions Discord Bot Help",
		Color: 0x0099ff,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Text Commands",
				Value: "`!gh workflows <owner> <repo>` - List workflows\n`!gh run <owner> <repo> <workflow_id>` - Run workflow\n`!gh status <owner> <repo>` - Get workflow status\n`!gh logs <owner> <repo> <run_id>` - Get workflow logs",
			},
			{
				Name:  "Slash Commands",
				Value: "`/build` - Run all build workflows\n`/build-win` - Run Windows build\n`/build-mac` - Run macOS build\n`/build-drive` - Run AssetImporter build\n`/code-check` - Run static analysis",
			},
		},
	}
	s.ChannelMessageSendEmbed(channelID, embed)
}

func (b *Bot) registerSlashCommands() {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "build",
			Description: "Run all build workflows",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "owner",
					Description: "Repository owner",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "repo",
					Description: "Repository name",
					Required:    true,
				},
			},
		},
		{
			Name:        "build-win",
			Description: "Run Windows build workflow",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "owner",
					Description: "Repository owner",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "repo",
					Description: "Repository name",
					Required:    true,
				},
			},
		},
		{
			Name:        "build-mac",
			Description: "Run macOS build workflow",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "owner",
					Description: "Repository owner",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "repo",
					Description: "Repository name",
					Required:    true,
				},
			},
		},
		{
			Name:        "build-drive",
			Description: "Run AssetImporter build workflow",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "owner",
					Description: "Repository owner",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "repo",
					Description: "Repository name",
					Required:    true,
				},
			},
		},
		{
			Name:        "code-check",
			Description: "Run static code analysis workflow",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "owner",
					Description: "Repository owner",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "repo",
					Description: "Repository name",
					Required:    true,
				},
			},
		},
	}

	for _, cmd := range commands {
		_, err := b.discord.ApplicationCommandCreate(b.discord.State.User.ID, b.guildID, cmd)
		if err != nil {
			log.Printf("Cannot create '%v' command: %v", cmd.Name, err)
		}
	}
}

func (b *Bot) interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.ApplicationCommandData().Name == "" {
		return
	}

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	owner := optionMap["owner"].StringValue()
	repo := optionMap["repo"].StringValue()

	switch i.ApplicationCommandData().Name {
	case "build":
		b.handleBuildCommand(s, i, owner, repo, "build")
	case "build-win":
		b.handleBuildCommand(s, i, owner, repo, "build-windows")
	case "build-mac":
		b.handleBuildCommand(s, i, owner, repo, "build-macos")
	case "build-drive":
		b.handleBuildCommand(s, i, owner, repo, "build-drive")
	case "code-check":
		b.handleBuildCommand(s, i, owner, repo, "code-check")
	}
}

func (b *Bot) handleBuildCommand(s *discordgo.Session, i *discordgo.InteractionCreate, owner, repo, workflowName string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Starting %s workflow for %s/%s...", workflowName, owner, repo),
		},
	})
	if err != nil {
		log.Printf("Error responding to interaction: %v", err)
		return
	}

	workflows, _, err := b.github.Actions.ListWorkflows(context.Background(), owner, repo, nil)
	if err != nil {
		b.followUpError(s, i, fmt.Sprintf("Error listing workflows: %v", err))
		return
	}

	var targetWorkflow *github.Workflow
	for _, workflow := range workflows.Workflows {
		if strings.Contains(strings.ToLower(*workflow.Name), workflowName) {
			targetWorkflow = workflow
			break
		}
	}

	if targetWorkflow == nil {
		b.followUpError(s, i, fmt.Sprintf("Workflow '%s' not found", workflowName))
		return
	}

	event := github.CreateWorkflowDispatchEventRequest{
		Ref: "main",
	}

	_, err = b.github.Actions.CreateWorkflowDispatchEventByID(
		context.Background(), owner, repo, *targetWorkflow.ID, event,
	)
	if err != nil {
		b.followUpError(s, i, fmt.Sprintf("Error running workflow: %v", err))
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "Workflow Started",
		Description: fmt.Sprintf("Workflow '%s' in %s/%s has been triggered", *targetWorkflow.Name, owner, repo),
		Color:       0x00ff00,
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
	if err != nil {
		log.Printf("Error sending followup message: %v", err)
	}
}

func (b *Bot) followUpError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: message,
	})
	if err != nil {
		log.Printf("Error sending error followup: %v", err)
	}
}

func (b *Bot) ready(s *discordgo.Session, event *discordgo.Ready) {
	log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
}
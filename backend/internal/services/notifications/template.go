package notifications

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yourusername/streammaxing/internal/db"
	discordSvc "github.com/yourusername/streammaxing/internal/services/discord"
	twitchSvc "github.com/yourusername/streammaxing/internal/services/twitch"
)

// TemplateService handles rendering message templates with dynamic data
type TemplateService struct{}

// NewTemplateService creates a new template service
func NewTemplateService() *TemplateService {
	return &TemplateService{}
}

// RenderTemplate renders a message template with streamer and stream data
func (s *TemplateService) RenderTemplate(
	templateJSON json.RawMessage,
	streamer *db.Streamer,
	streamData *twitchSvc.StreamData,
	mentionRoleID string,
) (*discordSvc.DiscordMessage, error) {
	var tmpl db.MessageTemplate
	if err := json.Unmarshal(templateJSON, &tmpl); err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	// Build variable map
	vars := map[string]string{
		"{streamer_login}":        streamer.TwitchLogin,
		"{streamer_display_name}": streamer.TwitchDisplayName,
		"{streamer_avatar_url}":   streamer.TwitchAvatarURL,
		"{stream_title}":          streamData.Title,
		"{game_name}":             streamData.GameName,
		"{viewer_count}":          fmt.Sprintf("%d", streamData.ViewerCount),
		"{stream_thumbnail_url}":  strings.ReplaceAll(streamData.ThumbnailURL, "{width}x{height}", "1920x1080"),
		"{started_at}":            streamData.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	// Add mention role
	if mentionRoleID != "" {
		vars["{mention_role}"] = fmt.Sprintf("<@&%s>", mentionRoleID)
	} else {
		vars["{mention_role}"] = ""
	}

	// Render content
	content := replaceVariables(tmpl.Content, vars)

	// Render embed
	var embeds []*discordSvc.DiscordEmbed
	if tmpl.Embed != nil {
		embed := &discordSvc.DiscordEmbed{
			Title:       replaceVariables(tmpl.Embed.Title, vars),
			Description: replaceVariables(tmpl.Embed.Description, vars),
			URL:         replaceVariables(tmpl.Embed.URL, vars),
			Color:       tmpl.Embed.Color,
		}

		if tmpl.Embed.Thumbnail != nil {
			embed.Thumbnail = &discordSvc.DiscordImage{
				URL: replaceVariables(tmpl.Embed.Thumbnail.URL, vars),
			}
		}

		if tmpl.Embed.Image != nil {
			embed.Image = &discordSvc.DiscordImage{
				URL: replaceVariables(tmpl.Embed.Image.URL, vars),
			}
		}

		for _, field := range tmpl.Embed.Fields {
			embed.Fields = append(embed.Fields, discordSvc.DiscordField{
				Name:   replaceVariables(field.Name, vars),
				Value:  replaceVariables(field.Value, vars),
				Inline: field.Inline,
			})
		}

		if tmpl.Embed.Footer != nil {
			embed.Footer = &discordSvc.DiscordFooter{
				Text: replaceVariables(tmpl.Embed.Footer.Text, vars),
			}
		}

		if tmpl.Embed.Timestamp {
			embed.Timestamp = streamData.StartedAt.Format("2006-01-02T15:04:05Z07:00")
		}

		embeds = append(embeds, embed)
	}

	return &discordSvc.DiscordMessage{
		Content: content,
		Embeds:  embeds,
	}, nil
}

// RenderCustomContent renders a plain text string with template variables
func (s *TemplateService) RenderCustomContent(
	content string,
	streamer *db.Streamer,
	streamData *twitchSvc.StreamData,
	mentionRoleID string,
) string {
	vars := map[string]string{
		"{streamer_login}":        streamer.TwitchLogin,
		"{streamer_display_name}": streamer.TwitchDisplayName,
		"{streamer_avatar_url}":   streamer.TwitchAvatarURL,
		"{stream_title}":          streamData.Title,
		"{game_name}":             streamData.GameName,
		"{viewer_count}":          fmt.Sprintf("%d", streamData.ViewerCount),
	}
	if mentionRoleID != "" {
		vars["{mention_role}"] = fmt.Sprintf("<@&%s>", mentionRoleID)
	} else {
		vars["{mention_role}"] = ""
	}
	return replaceVariables(content, vars)
}

// replaceVariables replaces template variables with their values
func replaceVariables(text string, vars map[string]string) string {
	for key, value := range vars {
		text = strings.ReplaceAll(text, key, value)
	}
	return text
}

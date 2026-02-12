package pijul

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

// Channel represents a Pijul channel (analogous to a Git branch)
type Channel struct {
	Name      string `json:"name"`
	IsCurrent bool   `json:"is_current,omitempty"`
}

// Channels returns the list of channels in the repository
func (p *PijulRepo) Channels() ([]Channel, error) {
	output, err := p.channelCmd()
	if err != nil {
		return nil, fmt.Errorf("listing channels: %w", err)
	}

	return parseChannelOutput(output)
}

// parseChannelOutput parses the output of pijul channel
// Expected format:
//
//	* main
//	  feature-branch
//	  another-branch
func parseChannelOutput(output []byte) ([]Channel, error) {
	var channels []Channel
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		isCurrent := strings.HasPrefix(line, "* ")
		name := strings.TrimSpace(strings.TrimPrefix(line, "* "))
		name = strings.TrimSpace(strings.TrimPrefix(name, "  "))

		if name != "" {
			channels = append(channels, Channel{
				Name:      name,
				IsCurrent: isCurrent,
			})
		}
	}

	return channels, scanner.Err()
}

// ChannelOptions options for channel listing
type ChannelOptions struct {
	Limit  int
	Offset int
}

// ChannelsWithOptions returns channels with pagination
func (p *PijulRepo) ChannelsWithOptions(opts *ChannelOptions) ([]Channel, error) {
	channels, err := p.Channels()
	if err != nil {
		return nil, err
	}

	if opts == nil {
		return channels, nil
	}

	// Apply offset
	if opts.Offset > 0 {
		if opts.Offset >= len(channels) {
			return []Channel{}, nil
		}
		channels = channels[opts.Offset:]
	}

	// Apply limit
	if opts.Limit > 0 && opts.Limit < len(channels) {
		channels = channels[:opts.Limit]
	}

	return channels, nil
}

// CreateChannel creates a new channel
func (p *PijulRepo) CreateChannel(name string) error {
	_, err := p.channelCmd("new", name)
	return err
}

// DeleteChannel deletes a channel
func (p *PijulRepo) DeleteChannel(name string) error {
	_, err := p.channelCmd("delete", name)
	return err
}

// RenameChannel renames a channel
func (p *PijulRepo) RenameChannel(oldName, newName string) error {
	_, err := p.channelCmd("rename", oldName, newName)
	return err
}

// SwitchChannel switches to a different channel
func (p *PijulRepo) SwitchChannel(name string) error {
	_, err := p.channelCmd("switch", name)
	if err != nil {
		return fmt.Errorf("switching to channel %s: %w", name, err)
	}
	p.channelName = name
	return nil
}

// CurrentChannelName returns the name of the current channel
func (p *PijulRepo) CurrentChannelName() (string, error) {
	channels, err := p.Channels()
	if err != nil {
		return "", err
	}

	for _, ch := range channels {
		if ch.IsCurrent {
			return ch.Name, nil
		}
	}

	// If no current channel marked, default to "main"
	return "main", nil
}

// ForkChannel creates a new channel from an existing one
// This is equivalent to Git's "git checkout -b newbranch oldbranch"
func (p *PijulRepo) ForkChannel(newName, fromChannel string) error {
	args := []string{"fork", newName}
	if fromChannel != "" {
		args = append(args, "--channel", fromChannel)
	}
	_, err := p.channelCmd(args...)
	return err
}

// ChannelExists checks if a channel exists
func (p *PijulRepo) ChannelExists(name string) (bool, error) {
	channels, err := p.Channels()
	if err != nil {
		return false, err
	}

	for _, ch := range channels {
		if ch.Name == name {
			return true, nil
		}
	}

	return false, nil
}

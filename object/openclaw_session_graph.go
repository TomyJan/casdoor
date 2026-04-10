// Copyright 2026 The Casdoor Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package object

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/casdoor/casdoor/util"
)

type OpenClawSessionGraph struct {
	Nodes []*OpenClawSessionGraphNode `json:"nodes"`
	Edges []*OpenClawSessionGraphEdge `json:"edges"`
	Stats OpenClawSessionGraphStats   `json:"stats"`
}

type OpenClawSessionGraphNode struct {
	ID               string `json:"id"`
	ParentID         string `json:"parentId,omitempty"`
	OriginalParentID string `json:"originalParentId,omitempty"`
	EntryID          string `json:"entryId,omitempty"`
	ToolCallID       string `json:"toolCallId,omitempty"`
	Kind             string `json:"kind"`
	Timestamp        string `json:"timestamp"`
	Summary          string `json:"summary"`
	Tool             string `json:"tool,omitempty"`
	Query            string `json:"query,omitempty"`
	URL              string `json:"url,omitempty"`
	Path             string `json:"path,omitempty"`
	OK               *bool  `json:"ok,omitempty"`
	Error            string `json:"error,omitempty"`
	Text             string `json:"text,omitempty"`
	IsAnchor         bool   `json:"isAnchor"`
}

type OpenClawSessionGraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type OpenClawSessionGraphStats struct {
	TotalNodes      int `json:"totalNodes"`
	TaskCount       int `json:"taskCount"`
	ToolCallCount   int `json:"toolCallCount"`
	ToolResultCount int `json:"toolResultCount"`
	FinalCount      int `json:"finalCount"`
	FailedCount     int `json:"failedCount"`
}

type openClawSessionGraphBuilder struct {
	graph *OpenClawSessionGraph
	nodes map[string]*OpenClawSessionGraphNode
}

func GetOpenClawSessionGraph(id string) (*OpenClawSessionGraph, error) {
	entry, err := GetEntry(id)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	provider, err := GetProvider(util.GetId(entry.Owner, entry.Provider))
	if err != nil {
		return nil, err
	}
	if !isOpenClawLogProvider(provider) || strings.TrimSpace(entry.Type) != "session" {
		return nil, fmt.Errorf("entry %s is not an OpenClaw session entry", id)
	}

	anchorPayload, err := parseOpenClawSessionGraphPayload(entry)
	if err != nil {
		return nil, fmt.Errorf("failed to parse anchor entry %s: %w", entry.Name, err)
	}

	rawGraph, err := tryBuildOpenClawSessionGraphFromRaw(provider, anchorPayload)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to build OpenClaw session graph from raw transcript: %w", err)
	}
	return rawGraph, nil
}

func parseOpenClawSessionGraphPayload(entry *Entry) (openClawBehaviorPayload, error) {
	if entry == nil {
		return openClawBehaviorPayload{}, fmt.Errorf("entry is nil")
	}

	message := strings.TrimSpace(entry.Message)
	if message == "" {
		return openClawBehaviorPayload{}, fmt.Errorf("message is empty")
	}

	var payload openClawBehaviorPayload
	if err := json.Unmarshal([]byte(message), &payload); err != nil {
		return openClawBehaviorPayload{}, err
	}

	payload.SessionID = strings.TrimSpace(payload.SessionID)
	payload.EntryID = strings.TrimSpace(payload.EntryID)
	payload.ParentID = strings.TrimSpace(payload.ParentID)
	payload.Kind = strings.TrimSpace(payload.Kind)
	payload.Summary = strings.TrimSpace(payload.Summary)
	payload.Tool = strings.TrimSpace(payload.Tool)
	payload.Query = strings.TrimSpace(payload.Query)
	payload.URL = strings.TrimSpace(payload.URL)
	payload.Path = strings.TrimSpace(payload.Path)
	payload.Error = strings.TrimSpace(payload.Error)
	payload.Text = strings.TrimSpace(payload.Text)
	payload.Timestamp = strings.TrimSpace(firstNonEmpty(payload.Timestamp, entry.CreatedTime))

	if payload.SessionID == "" {
		return openClawBehaviorPayload{}, fmt.Errorf("sessionId is empty")
	}
	if payload.EntryID == "" {
		return openClawBehaviorPayload{}, fmt.Errorf("entryId is empty")
	}
	if payload.Kind == "" {
		return openClawBehaviorPayload{}, fmt.Errorf("kind is empty")
	}

	return payload, nil
}

func tryBuildOpenClawSessionGraphFromRaw(provider *Provider, anchorPayload openClawBehaviorPayload) (*OpenClawSessionGraph, error) {
	transcriptPath, err := resolveOpenClawSessionTranscriptPath(provider, anchorPayload.SessionID)
	if err != nil {
		return nil, err
	}

	entries, err := loadOpenClawSessionTranscriptEntries(transcriptPath)
	if err != nil {
		return nil, err
	}

	return buildOpenClawSessionGraphFromRaw(anchorPayload, entries), nil
}

func resolveOpenClawSessionTranscriptPath(provider *Provider, sessionID string) (string, error) {
	transcriptDir, err := resolveOpenClawTranscriptDir(provider)
	if err != nil {
		return "", err
	}

	sessionID, err = validateOpenClawSessionID(sessionID)
	if err != nil {
		return "", err
	}

	path, err := joinPathWithinDir(transcriptDir, fmt.Sprintf("%s.jsonl", sessionID))
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}

		fallbackPath, fallbackErr := resolveLatestOpenClawResetTranscriptPath(transcriptDir, sessionID)
		if fallbackErr == nil {
			return fallbackPath, nil
		}
		if os.IsNotExist(fallbackErr) {
			return "", fmt.Errorf("session file %s does not exist: %w", path, os.ErrNotExist)
		}
		return "", fallbackErr
	}
	if info.IsDir() {
		return "", fmt.Errorf("session path %s is a directory", path)
	}
	return path, nil
}

func validateOpenClawSessionID(sessionID string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", fmt.Errorf("sessionId is empty")
	}
	if filepath.IsAbs(sessionID) {
		return "", fmt.Errorf("invalid sessionId: absolute path is not allowed")
	}
	if sessionID == "." || sessionID == ".." {
		return "", fmt.Errorf("invalid sessionId: relative path segments are not allowed")
	}
	if strings.Contains(sessionID, "/") || strings.Contains(sessionID, "\\") {
		return "", fmt.Errorf("invalid sessionId: path separators are not allowed")
	}
	if filepath.Clean(sessionID) != sessionID {
		return "", fmt.Errorf("invalid sessionId: normalized value mismatch")
	}
	if filepath.Base(sessionID) != sessionID {
		return "", fmt.Errorf("invalid sessionId: basename mismatch")
	}
	return sessionID, nil
}

func joinPathWithinDir(baseDir, fileName string) (string, error) {
	baseDir = filepath.Clean(baseDir)
	candidate := filepath.Clean(filepath.Join(baseDir, fileName))

	relativePath, err := filepath.Rel(baseDir, candidate)
	if err != nil {
		return "", err
	}
	if relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) || filepath.IsAbs(relativePath) {
		return "", fmt.Errorf("resolved path escaped transcript directory")
	}

	return candidate, nil
}

func resolveLatestOpenClawResetTranscriptPath(transcriptDir, sessionID string) (string, error) {
	entries, err := os.ReadDir(transcriptDir)
	if err != nil {
		return "", err
	}

	prefix := fmt.Sprintf("%s.jsonl.reset.", strings.TrimSpace(sessionID))
	type transcriptCandidate struct {
		name      string
		path      string
		modTimeNs int64
	}

	candidates := []transcriptCandidate{}
	for _, entry := range entries {
		if entry == nil || entry.IsDir() {
			continue
		}

		name := strings.TrimSpace(entry.Name())
		if !strings.HasPrefix(name, prefix) {
			continue
		}

		candidate := transcriptCandidate{
			name: name,
		}
		candidate.path, err = joinPathWithinDir(transcriptDir, name)
		if err != nil {
			continue
		}
		if info, infoErr := entry.Info(); infoErr == nil && info != nil {
			candidate.modTimeNs = info.ModTime().UnixNano()
		}
		candidates = append(candidates, candidate)
	}

	if len(candidates) == 0 {
		return "", os.ErrNotExist
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].name != candidates[j].name {
			return candidates[i].name > candidates[j].name
		}
		return candidates[i].modTimeNs > candidates[j].modTimeNs
	})

	return candidates[0].path, nil
}

func loadOpenClawSessionTranscriptEntries(path string) ([]openClawTranscriptEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	entries := []openClawTranscriptEntry{}
	reader := bufio.NewReader(file)
	for {
		lineBytes, readErr := reader.ReadBytes('\n')
		if readErr != nil && readErr != io.EOF {
			return nil, readErr
		}

		line := strings.TrimSpace(string(lineBytes))
		if line != "" {
			var entry openClawTranscriptEntry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				continue
			}
			entries = append(entries, entry)
		}

		if readErr == io.EOF {
			break
		}
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("session file %s did not contain any valid transcript entries", path)
	}

	return entries, nil
}

func buildOpenClawSessionGraphFromRaw(anchorPayload openClawBehaviorPayload, transcriptEntries []openClawTranscriptEntry) *OpenClawSessionGraph {
	builder := newOpenClawSessionGraphBuilder()
	toolContexts := map[string]openClawToolContext{}
	rawToolCallsByAssistant := map[string][]string{}

	for _, entry := range transcriptEntries {
		if entry.Type != "message" || entry.Message == nil {
			continue
		}

		message := entry.Message
		switch message.Role {
		case "user":
			text := normalizeUserText(extractMessageText(message.Content))
			if text == "" {
				continue
			}
			builder.addNode(&OpenClawSessionGraphNode{
				ID:        strings.TrimSpace(entry.ID),
				ParentID:  strings.TrimSpace(entry.ParentID),
				EntryID:   strings.TrimSpace(entry.ID),
				Kind:      "task",
				Timestamp: normalizeOpenClawTimestamp(entry.Timestamp, message.Timestamp),
				Summary:   truncateText(fmt.Sprintf("task: %s", text), 100),
				Text:      truncateText(text, 2000),
			})
		case "assistant":
			items := parseContentItems(message.Content)
			toolNames := []string{}
			hasToolCalls := false
			for index, item := range items {
				if item.Type != "toolCall" {
					continue
				}

				hasToolCalls = true
				context := extractOpenClawToolContext(item)
				if item.ID != "" {
					toolContexts[item.ID] = context
				}

				toolCallID := strings.TrimSpace(item.ID)
				nodeID := buildRawToolCallNodeID(strings.TrimSpace(entry.ID), toolCallID, index)
				builder.addNode(&OpenClawSessionGraphNode{
					ID:         nodeID,
					ParentID:   strings.TrimSpace(entry.ID),
					EntryID:    strings.TrimSpace(entry.ID),
					ToolCallID: toolCallID,
					Kind:       "tool_call",
					Timestamp:  normalizeOpenClawTimestamp(entry.Timestamp, message.Timestamp),
					Summary:    truncateText(buildToolCallSummary(context), 100),
					Tool:       context.Tool,
					Query:      context.Query,
					URL:        context.URL,
					Path:       context.Path,
					Text:       truncateText(context.Command, 500),
				})
				rawToolCallsByAssistant[strings.TrimSpace(entry.ID)] = append(rawToolCallsByAssistant[strings.TrimSpace(entry.ID)], nodeID)
				toolNames = append(toolNames, context.Tool)
			}

			if hasToolCalls {
				builder.addNode(&OpenClawSessionGraphNode{
					ID:        strings.TrimSpace(entry.ID),
					ParentID:  strings.TrimSpace(entry.ParentID),
					EntryID:   strings.TrimSpace(entry.ID),
					Kind:      "assistant_step",
					Timestamp: normalizeOpenClawTimestamp(entry.Timestamp, message.Timestamp),
					Summary:   buildAssistantStepSummary(toolNames),
					Text:      truncateText(extractMessageText(message.Content), 2000),
				})
				continue
			}

			if message.StopReason != "stop" {
				continue
			}
			text := extractMessageText(message.Content)
			if text == "" {
				continue
			}
			builder.addNode(&OpenClawSessionGraphNode{
				ID:        strings.TrimSpace(entry.ID),
				ParentID:  strings.TrimSpace(entry.ParentID),
				EntryID:   strings.TrimSpace(entry.ID),
				Kind:      "final",
				Timestamp: normalizeOpenClawTimestamp(entry.Timestamp, message.Timestamp),
				Summary:   truncateText(fmt.Sprintf("final: %s", text), 100),
				Text:      truncateText(text, 2000),
			})
		case "toolResult":
			payload, ok := buildToolResultPayload(anchorPayload.SessionID, entry, toolContexts[message.ToolCallID])
			if !ok {
				continue
			}

			parentID := strings.TrimSpace(entry.ParentID)
			originalParentID := ""
			if message.ToolCallID != "" {
				for _, candidateID := range rawToolCallsByAssistant[parentID] {
					candidate := builder.nodes[candidateID]
					if candidate != nil && candidate.ToolCallID == strings.TrimSpace(message.ToolCallID) {
						originalParentID = parentID
						parentID = candidateID
						break
					}
				}

				if parentID == strings.TrimSpace(entry.ParentID) {
					for _, candidate := range builder.nodes {
						if candidate.Kind == "tool_call" && candidate.ToolCallID == strings.TrimSpace(message.ToolCallID) {
							originalParentID = strings.TrimSpace(entry.ParentID)
							parentID = candidate.ID
							break
						}
					}
				}
			}

			builder.addNode(&OpenClawSessionGraphNode{
				ID:               strings.TrimSpace(entry.ID),
				ParentID:         parentID,
				OriginalParentID: originalParentID,
				EntryID:          strings.TrimSpace(entry.ID),
				ToolCallID:       strings.TrimSpace(message.ToolCallID),
				Kind:             "tool_result",
				Timestamp:        payload.Timestamp,
				Summary:          payload.Summary,
				Tool:             payload.Tool,
				Query:            payload.Query,
				URL:              payload.URL,
				Path:             payload.Path,
				OK:               cloneBoolPointer(payload.OK),
				Error:            payload.Error,
				Text:             payload.Text,
			})
		}
	}

	markRawGraphAnchor(builder, anchorPayload)
	return builder.finalize()
}

func markRawGraphAnchor(builder *openClawSessionGraphBuilder, anchorPayload openClawBehaviorPayload) {
	anchorNodeID := ""

	switch anchorPayload.Kind {
	case "tool_call":
		candidates := []string{}
		for _, node := range builder.nodes {
			if !toolCallPayloadMatchesNode(anchorPayload, node) {
				continue
			}
			candidates = append(candidates, node.ID)
		}

		switch len(candidates) {
		case 1:
			anchorNodeID = candidates[0]
		default:
			anchorNodeID = anchorPayload.EntryID
		}
	default:
		if node := builder.nodes[anchorPayload.EntryID]; node != nil && node.Kind == anchorPayload.Kind {
			anchorNodeID = node.ID
		}
	}

	if anchorNode := builder.nodes[anchorNodeID]; anchorNode != nil {
		anchorNode.IsAnchor = true
	}
}

func newOpenClawSessionGraphBuilder() *openClawSessionGraphBuilder {
	return &openClawSessionGraphBuilder{
		graph: &OpenClawSessionGraph{
			Nodes: []*OpenClawSessionGraphNode{},
			Edges: []*OpenClawSessionGraphEdge{},
		},
		nodes: map[string]*OpenClawSessionGraphNode{},
	}
}

func (b *openClawSessionGraphBuilder) addNode(node *OpenClawSessionGraphNode) {
	if b == nil || node == nil {
		return
	}

	node.ID = strings.TrimSpace(node.ID)
	if node.ID == "" {
		return
	}

	if existing := b.nodes[node.ID]; existing != nil {
		mergeOpenClawGraphNode(existing, node)
		return
	}

	cloned := *node
	cloned.ParentID = strings.TrimSpace(cloned.ParentID)
	cloned.OriginalParentID = strings.TrimSpace(cloned.OriginalParentID)
	cloned.EntryID = strings.TrimSpace(cloned.EntryID)
	cloned.ToolCallID = strings.TrimSpace(cloned.ToolCallID)
	cloned.Kind = strings.TrimSpace(cloned.Kind)
	cloned.Timestamp = strings.TrimSpace(cloned.Timestamp)
	cloned.Summary = strings.TrimSpace(cloned.Summary)
	cloned.Tool = strings.TrimSpace(cloned.Tool)
	cloned.Query = strings.TrimSpace(cloned.Query)
	cloned.URL = strings.TrimSpace(cloned.URL)
	cloned.Path = strings.TrimSpace(cloned.Path)
	cloned.Error = strings.TrimSpace(cloned.Error)
	cloned.Text = strings.TrimSpace(cloned.Text)
	cloned.OK = cloneBoolPointer(cloned.OK)
	b.nodes[cloned.ID] = &cloned
}

func (b *openClawSessionGraphBuilder) finalize() *OpenClawSessionGraph {
	if b == nil || b.graph == nil {
		return nil
	}

	nodeIDs := make([]string, 0, len(b.nodes))
	for id := range b.nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Slice(nodeIDs, func(i, j int) bool {
		left := b.nodes[nodeIDs[i]]
		right := b.nodes[nodeIDs[j]]
		return compareGraphNodes(left, right) < 0
	})

	b.graph.Nodes = make([]*OpenClawSessionGraphNode, 0, len(nodeIDs))
	b.graph.Stats = OpenClawSessionGraphStats{}
	for _, id := range nodeIDs {
		node := b.nodes[id]
		b.graph.Nodes = append(b.graph.Nodes, node)
		updateOpenClawSessionGraphStats(&b.graph.Stats, node)
	}

	edgeKeys := map[string]struct{}{}
	b.graph.Edges = []*OpenClawSessionGraphEdge{}
	for _, node := range b.graph.Nodes {
		if node.ParentID == "" || b.nodes[node.ParentID] == nil {
			continue
		}
		key := fmt.Sprintf("%s->%s", node.ParentID, node.ID)
		if _, ok := edgeKeys[key]; ok {
			continue
		}
		edgeKeys[key] = struct{}{}
		b.graph.Edges = append(b.graph.Edges, &OpenClawSessionGraphEdge{
			Source: node.ParentID,
			Target: node.ID,
		})
	}

	sort.Slice(b.graph.Edges, func(i, j int) bool {
		left := b.graph.Edges[i]
		right := b.graph.Edges[j]
		if left.Source != right.Source {
			return left.Source < right.Source
		}
		return left.Target < right.Target
	})

	return b.graph
}

func mergeOpenClawGraphNode(current, next *OpenClawSessionGraphNode) {
	if current == nil || next == nil {
		return
	}

	current.ParentID = firstNonEmpty(current.ParentID, next.ParentID)
	current.OriginalParentID = firstNonEmpty(current.OriginalParentID, next.OriginalParentID)
	current.EntryID = firstNonEmpty(current.EntryID, next.EntryID)
	current.ToolCallID = firstNonEmpty(current.ToolCallID, next.ToolCallID)
	current.Kind = firstNonEmpty(current.Kind, next.Kind)
	current.Timestamp = chooseEarlierTimestamp(current.Timestamp, next.Timestamp)
	current.Summary = firstNonEmpty(current.Summary, next.Summary)
	current.Tool = firstNonEmpty(current.Tool, next.Tool)
	current.Query = firstNonEmpty(current.Query, next.Query)
	current.URL = firstNonEmpty(current.URL, next.URL)
	current.Path = firstNonEmpty(current.Path, next.Path)
	current.Error = firstNonEmpty(current.Error, next.Error)
	current.Text = firstNonEmpty(current.Text, next.Text)
	current.OK = mergeBoolPointers(current.OK, next.OK)
	current.IsAnchor = current.IsAnchor || next.IsAnchor
}

func updateOpenClawSessionGraphStats(stats *OpenClawSessionGraphStats, node *OpenClawSessionGraphNode) {
	if stats == nil || node == nil {
		return
	}

	stats.TotalNodes++
	switch node.Kind {
	case "task":
		stats.TaskCount++
	case "tool_call":
		stats.ToolCallCount++
	case "tool_result":
		stats.ToolResultCount++
		if node.OK != nil && !*node.OK {
			stats.FailedCount++
		}
	case "final":
		stats.FinalCount++
	}
}

func buildRawToolCallNodeID(entryID, toolCallID string, index int) string {
	entryID = strings.TrimSpace(entryID)
	toolCallID = strings.TrimSpace(toolCallID)
	if toolCallID != "" {
		return fmt.Sprintf("tool_call:%s", toolCallID)
	}
	return fmt.Sprintf("tool_call:%s:%d", entryID, index)
}

func buildAssistantStepSummary(toolNames []string) string {
	deduped := []string{}
	seen := map[string]struct{}{}
	for _, toolName := range toolNames {
		toolName = strings.TrimSpace(toolName)
		if toolName == "" {
			continue
		}
		if _, ok := seen[toolName]; ok {
			continue
		}
		seen[toolName] = struct{}{}
		deduped = append(deduped, toolName)
	}

	if len(toolNames) == 0 {
		return "assistant step"
	}
	if len(deduped) == 0 {
		return fmt.Sprintf("%d tool calls", len(toolNames))
	}
	if len(deduped) <= 3 {
		return fmt.Sprintf("%d tool calls: %s", len(toolNames), strings.Join(deduped, ", "))
	}
	return fmt.Sprintf("%d tool calls: %s, ...", len(toolNames), strings.Join(deduped[:3], ", "))
}

func toolCallPayloadMatchesNode(payload openClawBehaviorPayload, node *OpenClawSessionGraphNode) bool {
	if node == nil || node.Kind != "tool_call" {
		return false
	}
	if strings.TrimSpace(node.EntryID) != strings.TrimSpace(payload.EntryID) {
		return false
	}

	fields := []struct {
		payload string
		node    string
	}{
		{payload.Tool, node.Tool},
		{payload.Query, node.Query},
		{payload.URL, node.URL},
		{payload.Path, node.Path},
		{payload.Text, node.Text},
	}

	matchedField := false
	for _, field := range fields {
		left := strings.TrimSpace(field.payload)
		if left == "" {
			continue
		}
		matchedField = true
		if left != strings.TrimSpace(field.node) {
			return false
		}
	}

	return matchedField
}

func compareGraphNodes(left, right *OpenClawSessionGraphNode) int {
	leftTimestamp := ""
	rightTimestamp := ""
	leftID := ""
	rightID := ""
	if left != nil {
		leftTimestamp = left.Timestamp
		leftID = left.ID
	}
	if right != nil {
		rightTimestamp = right.Timestamp
		rightID = right.ID
	}
	if leftTimestamp < rightTimestamp {
		return -1
	}
	if leftTimestamp > rightTimestamp {
		return 1
	}
	if leftID < rightID {
		return -1
	}
	if leftID > rightID {
		return 1
	}
	return 0
}

func chooseEarlierTimestamp(current, next string) string {
	current = strings.TrimSpace(current)
	next = strings.TrimSpace(next)
	if current == "" {
		return next
	}
	if next == "" {
		return current
	}
	if next < current {
		return next
	}
	return current
}

func mergeBoolPointers(current, next *bool) *bool {
	if next == nil {
		return current
	}
	if current == nil {
		return cloneBoolPointer(next)
	}
	value := *current && *next
	return &value
}

func cloneBoolPointer(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

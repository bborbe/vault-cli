// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import libtime "github.com/bborbe/time"

// Decision represents a markdown file in the vault that has needs_review frontmatter.
//
// The embedded FrontmatterMap holds the complete frontmatter parsed from the
// file, including keys this type has no field for. WriteDecision serializes that
// map and overlays only the six managed keys, so a Trading Decision Record's
// selected_option, review_date, and related survive a read-write cycle untouched.
//
// Unlike Task/Goal/Theme/Objective/Vision, Decision keeps typed struct fields for
// its six managed keys instead of an XxxFrontmatter wrapper: those fields are the
// mutation surface pkg/ops/decision_ack.go writes and pkg/ops/decision_list.go
// reads directly.
type Decision struct {
	// FrontmatterMap holds every frontmatter key parsed from the file.
	// The yaml:"-" tag keeps it out of yaml.Marshal(Decision); gopkg.in/yaml.v3
	// does not inline anonymous struct fields, so without the tag it would emit
	// a stray "frontmattermap: {}" key.
	FrontmatterMap `yaml:"-"`

	// Managed frontmatter fields — the only six keys WriteDecision overlays.
	NeedsReview  bool                    `yaml:"needs_review"`
	Reviewed     bool                    `yaml:"reviewed,omitempty"`
	ReviewedDate *libtime.DateOrDateTime `yaml:"-"` // managed by storage layer
	Status       string                  `yaml:"status,omitempty"`
	Type         string                  `yaml:"type,omitempty"`
	PageType     string                  `yaml:"page_type,omitempty"`

	// Metadata — excluded from YAML serialization
	Name     string `yaml:"-"` // Relative path from vault root without .md extension
	Content  string `yaml:"-"` // Full markdown content including frontmatter
	FilePath string `yaml:"-"` // Absolute path to file
}

// NewDecision creates a Decision from a parsed frontmatter map and file metadata.
//
// The complete map is retained so unknown keys survive a read-write cycle; the
// six managed keys are additionally projected onto typed struct fields. Values
// are read through the coercing FrontmatterMap accessors, so a hand-quoted
// reviewed: "true" reads as true rather than silently as false.
func NewDecision(data map[string]any, name string, content string, filePath string) *Decision {
	fm := NewFrontmatterMap(data)
	decision := &Decision{
		FrontmatterMap: fm,
		NeedsReview:    fm.GetBool("needs_review"),
		Reviewed:       fm.GetBool("reviewed"),
		Status:         fm.GetString("status"),
		Type:           fm.GetString("type"),
		PageType:       fm.GetString("page_type"),
		Name:           name,
		Content:        content,
		FilePath:       filePath,
	}
	if t := fm.GetTime("reviewed_date"); t != nil {
		d := libtime.DateOrDateTime(*t)
		decision.ReviewedDate = &d
	}
	return decision
}

// DecisionID represents a decision identifier (relative vault path without .md extension).
type DecisionID string

func (d DecisionID) String() string {
	return string(d)
}

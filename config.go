package main

import (
	"github.com/opensourceways/community-robot-lib/config"
	"k8s.io/apimachinery/pkg/util/sets"
)

type configuration struct {
	ConfigItems []botConfig `json:"config_items,omitempty"`
}

func (c *configuration) configFor(org, repo string) *botConfig {
	if c == nil {
		return nil
	}

	items := c.ConfigItems
	v := make([]config.IRepoFilter, len(items))
	for i := range items {
		v[i] = &items[i]
	}

	if i := config.Find(org, repo, v); i >= 0 {
		return &items[i]
	}
	return nil
}

func (c *configuration) NeedLinkPullRequests(org, repo string) bool {
	if c == nil {
		return false
	}
	orgRepo := org + "/" + repo
	items := c.ConfigItems
	for _, item := range items {
		repoFilter := item.RepoFilter
		v := sets.NewString(repoFilter.Repos...)
		needLinkPullRequests := item.NeedIssueHasLinkPullRequests
		if v.Has(orgRepo) {
			return needLinkPullRequests
		}
		if !v.Has(org) {
			return false
		}
		if len(repoFilter.ExcludedRepos) > 0 && sets.NewString(repoFilter.ExcludedRepos...).Has(orgRepo) {
			return false
		}
		return needLinkPullRequests
	}
	return false
}

func (c *configuration) Validate() error {
	if c == nil {
		return nil
	}

	items := c.ConfigItems
	for i := range items {
		if err := items[i].validate(); err != nil {
			return err
		}
	}
	return nil
}

func (c *configuration) SetDefault() {
	if c == nil {
		return
	}

	Items := c.ConfigItems
	for i := range Items {
		Items[i].setDefault()
	}
}

type botConfig struct {
	config.RepoFilter
	NeedIssueHasLinkPullRequests bool `json:"need_issue_has_link_pull_requests,omitempty"`
}

func (c *botConfig) setDefault() {
}

func (c *botConfig) validate() error {
	return c.RepoFilter.Validate()
}

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/opensourceways/community-robot-lib/config"
	framework "github.com/opensourceways/community-robot-lib/robot-gitee-framework"
	sdk "github.com/opensourceways/go-gitee/gitee"
	"github.com/sirupsen/logrus"
)

const (
	botName                           = "lifecycle"
	issueNeedsLinkPullRequestsMessage = `***@%s*** you can't %s an issue unless the issue has link pull requests.`
)

var (
	reopenRe = regexp.MustCompile(`(?mi)^/reopen\s*$`)
	closeRe  = regexp.MustCompile(`(?mi)^/close\s*$`)
)

type iClient interface {
	CreateIssueComment(owner, repo string, number string, comment string) error
	HasLinkPullRequests(owner, repo, number string) (bool, error)
	GetIssueOperateLogs(owner, repo, number string) ([]sdk.OperateLog, error)
}

func newRobot(cli iClient) *robot {
	return &robot{cli: cli}
}

type robot struct {
	cli iClient
}

func (bot *robot) NewConfig() config.Config {
	return &configuration{}
}

func (bot *robot) getConfig(cfg config.Config) (*configuration, error) {
	if c, ok := cfg.(*configuration); ok {
		return c, nil
	}
	return nil, errors.New("can't convert to configuration")
}

func (bot *robot) RegisterEventHandler(p framework.HandlerRegister) {
	p.RegisterIssueHandler(bot.handleIssueStateChangeEvent)
}

func (bot *robot) handleIssueStateChangeEvent(e *sdk.IssueEvent, c config.Config, log *logrus.Entry) error {
	action := *e.Action
	state := *e.State
	if !(action == "state_change" && state == "closed") {
		return nil
	}
	org, repo := e.GetOrgRepo()
	number := e.GetIssueNumber()
	hasLinkPullRequests, err := bot.cli.HasLinkPullRequests(org, repo, number)
	if err != nil {
		logrus.Error("fail to check link pull requests when handle issue state change")
		return err
	}
	if hasLinkPullRequests {
		return nil
	}
	// 查询issue的操作日志，获取issue被修改前的状态
	operateLogs, err := bot.cli.GetIssueOperateLogs(org, repo, number)
	if err != nil {
		logrus.Error("fail to get operate logs of the issue")
		return err
	}
	operateSlices := strings.Split(operateLogs[0].Content, " ")
	issueState := operateSlices[len(operateSlices)-2]
	// 获取issue状态对应的issue_state_id
	issueStateId := getIssuesStatesId(issueState)
	if issueStateId == 0 {
		logrus.Error("fail to get issue state id")
		return nil
	}
	// 通过issue_state_id还原issue状态
	result, err := revertIssueState(e.Issue.Id, issueStateId)
	if err != nil {
		return err
	}
	if !result {
		logrus.Error("fail to revert issue state")
	} else {
		// 还原issue状态后评论
		return bot.cli.CreateIssueComment(org, repo, number, fmt.Sprintf(issueNeedsLinkPullRequestsMessage, e.User.Login, "close"))
	}
	return nil
}

func revertIssueState(issueId int32, issueStateId float64) (result bool, error error) {
	url := fmt.Sprintf("https://api.gitee.com/enterprises/%v/issues/%v", os.Getenv("enterpriseId"), issueId)
	payload := strings.NewReader(fmt.Sprintf("{\"access_token\": \"%v\", \"issue_state_id\": %v}", os.Getenv("v8AccessToken"), issueStateId))
	req, _ := http.NewRequest("PUT", url, payload)
	req.Header.Add("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logrus.Error("fail to revert issue state")
		return false, err
	}
	if resp.StatusCode != 200 {
		logrus.Error("get unexpected response when revert issue state")
		return false, nil
	}
	return true, nil
}

func getIssuesStatesId(stateName string) float64 {
	url := fmt.Sprintf("https://api.gitee.com/enterprises/%v/issue_states?access_token=%v", os.Getenv("enterpriseId"), os.Getenv("v8AccessToken"))
	fmt.Println("getIssuesStatesId: ", url)
	resp, err := http.Get(url)
	if err != nil {
		logrus.Error("fail to get enterprise issue states")
		return 0
	}
	if resp.StatusCode != 200 {
		logrus.Error("get unexpected response when getting enterprise pulls, status:", resp.Status)
		return 0
	}
	body, _ := ioutil.ReadAll(resp.Body)
	err = resp.Body.Close()
	if err != nil {
		logrus.Error("fail to close response body of enterprise pull requests, err：", err)
		return 0
	}
	statesSlicesResp := jsonToMap(string(body))
	if statesSlicesResp == nil {
		return 0
	}
	statesSlices := statesSlicesResp["data"].([]interface{})
	for _, issueState := range statesSlices {
		stateTitle := issueState.(map[string]interface{})["title"].(string)
		stateId := issueState.(map[string]interface{})["id"].(float64)
		if stateTitle == stateName {
			return stateId
		}
	}
	return 0
}

func jsonToMap(str string) map[string]interface{} {
	var tempMap map[string]interface{}
	err := json.Unmarshal([]byte(str), &tempMap)
	if err != nil {
		logrus.Error("Parse string to map error, the string is:", str)
		return nil
	}
	return tempMap
}

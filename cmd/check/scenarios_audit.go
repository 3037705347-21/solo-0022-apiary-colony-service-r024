package main

import (
	"fmt"

	"example.com/apiary-colony-service/internal/httpapi"
)

const (
	auditApiaryID = "apiary-north"
	auditHiveOne  = "hive-01"
	auditHiveTwo  = "hive-02"
)

func reqHeader(requestID string) map[string]string {
	return map[string]string{httpapi.RequestIDHeader: requestID}
}

// checkReadAuditEvents 走真实 HTTP 路径验证审计事件能力：
// 成功写入在蜂场/蜂箱范围可读、与业务结果同源；失败或被拒绝的请求不留下事件；
// 事件顺序与内容在重复读取间保持稳定。
func checkReadAuditEvents(c *client) error {
	// 非法请求编号头在进入业务前被拒绝，且不会创建蜂场。
	if _, err := c.expectWithHeaders("POST", "/apiaries", map[string]any{
		"id": "apiary-bad-header", "name": "坏编号", "district": "片区", "keeper": "值守人",
	}, reqHeader("bad id!"), 400, `"code":"invalid_request"`); err != nil {
		return err
	}

	// 成功录入蜂场：事件携带调用方请求编号，响应头原样回传。
	reply, err := c.expectWithHeaders("POST", "/apiaries", map[string]any{
		"id": auditApiaryID, "name": "北岭蜂场", "district": "北岭片区", "keeper": "周岚",
	}, reqHeader("req-apiary"), 201, `"id":"apiary-north"`)
	if err != nil {
		return err
	}
	if err := reply.mustHeader(httpapi.RequestIDHeader, "req-apiary"); err != nil {
		return err
	}

	if _, err := c.expectWithHeaders("POST", "/apiaries/"+auditApiaryID+"/hives", map[string]any{
		"id": auditHiveOne, "model": "langstroth",
	}, reqHeader("req-h01-install"), 201, `"state":"colonized"`); err != nil {
		return err
	}

	// 被拒绝的状态推进：迁移到自身、未知目标、production->colonized 回退。
	if _, err := c.expect("POST", "/hives/"+auditHiveOne+"/state", map[string]any{"target": "colonized"},
		409, `"code":"invalid_state"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/"+auditHiveOne+"/state", map[string]any{"target": "flying"},
		400, `"code":"invalid_request"`); err != nil {
		return err
	}
	if _, err := c.expectWithHeaders("POST", "/hives/"+auditHiveOne+"/state", map[string]any{"target": "production"},
		reqHeader("req-h01-prod"), 200, `"from":"colonized"`, `"to":"production"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/"+auditHiveOne+"/state", map[string]any{"target": "colonized"},
		409, `"code":"invalid_state"`); err != nil {
		return err
	}

	// 越界量测被整体拒绝，不应成为任何事实。
	if _, err := c.expect("POST", "/hives/"+auditHiveOne+"/inspections", map[string]any{
		"observedAt": "2026-05-05T05:05:05Z", "colonyStrength": 9, "broodPattern": "patchy",
		"temper": "calm", "miteCount": 3, "note": "应被拒绝",
	}, 400, `"code":"invalid_request"`); err != nil {
		return err
	}
	if _, err := c.expectWithHeaders("POST", "/hives/"+auditHiveOne+"/inspections", map[string]any{
		"observedAt": "2026-04-10T09:00:00Z", "colonyStrength": 3, "broodPattern": "patchy",
		"temper": "aggressive", "miteCount": 12, "note": "西侧巢门观察",
	}, reqHeader("req-h01-insp"), 201,
		`"inspection-000001"`, `"mite_load"`, `"temper_risk"`); err != nil {
		return err
	}
	if _, err := c.expectWithHeaders("POST", "/hives/"+auditHiveOne+"/state", map[string]any{"target": "retired"},
		reqHeader("req-h01-retire"), 200, `"from":"production"`, `"to":"retired"`); err != nil {
		return err
	}
	// 终态后推进、巡检、换王全部被拒绝。
	if _, err := c.expect("POST", "/hives/"+auditHiveOne+"/state", map[string]any{"target": "production"},
		409, `"code":"invalid_state"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/"+auditHiveOne+"/inspections", map[string]any{
		"observedAt": "2026-04-11T09:00:00Z", "colonyStrength": 3, "broodPattern": "solid",
		"temper": "calm", "miteCount": 1, "note": "",
	}, 409, `"code":"invalid_state"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/"+auditHiveOne+"/queens", map[string]any{
		"origin": "bred", "introducedAt": "2026-05-02T08:00:00Z", "reason": "loss",
	}, 409, `"code":"invalid_state"`); err != nil {
		return err
	}

	// 蜂场/蜂箱编号冲突与不存在，同样不产生事件。
	if _, err := c.expect("POST", "/apiaries", map[string]any{
		"id": auditApiaryID, "name": "重复蜂场", "district": "片区", "keeper": "值守人",
	}, 409, `"code":"conflict"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/apiaries/apiary-missing/hives", map[string]any{
		"id": "hive-09", "model": "warre",
	}, 404, `"code":"not_found"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/apiaries/"+auditApiaryID+"/hives", map[string]any{
		"id": "hive-bad", "model": "dadant",
	}, 400, `"code":"invalid_request"`); err != nil {
		return err
	}

	if _, err := c.expectWithHeaders("POST", "/apiaries/"+auditApiaryID+"/hives", map[string]any{
		"id": auditHiveTwo, "model": "warre",
	}, reqHeader("req-h02-install"), 201, `"state":"colonized"`); err != nil {
		return err
	}
	// 非法来源在领域校验阶段被拒绝。
	if _, err := c.expect("POST", "/hives/"+auditHiveTwo+"/queens", map[string]any{
		"origin": "imported", "introducedAt": "2026-04-01T08:00:00Z", "reason": "aging",
	}, 400, `"code":"invalid_request"`); err != nil {
		return err
	}
	if _, err := c.expectWithHeaders("POST", "/hives/"+auditHiveTwo+"/queens", map[string]any{
		"origin": "bred", "introducedAt": "2026-04-02T08:00:00Z", "reason": "aging",
	}, reqHeader("req-h02-q1"), 201, `"queen-000001"`, `"markColor":"white"`); err != nil {
		return err
	}
	if _, err := c.expectWithHeaders("POST", "/hives/"+auditHiveTwo+"/queens", map[string]any{
		"origin": "cells", "introducedAt": "2026-04-20T08:00:00Z", "reason": "aggression",
	}, reqHeader("req-h02-q2"), 201, `"queen-000002"`, `"markColor":"white"`); err != nil {
		return err
	}
	// 不带请求编号头：服务生成并回传，事件使用同一编号。
	retireHiveTwo, err := c.expect("POST", "/hives/"+auditHiveTwo+"/state", map[string]any{"target": "retired"},
		200, `"to":"retired"`)
	if err != nil {
		return err
	}
	generatedRequestID := retireHiveTwo.header.Get(httpapi.RequestIDHeader)
	if generatedRequestID == "" || generatedRequestID[:4] != "req-" {
		return fmt.Errorf("期望生成 req- 开头的请求编号，实际 %q", generatedRequestID)
	}
	if _, err := c.expect("POST", "/hives/"+auditHiveTwo+"/queens", map[string]any{
		"origin": "bred", "introducedAt": "2026-05-03T08:00:00Z", "reason": "aging",
	}, 409, `"code":"invalid_state"`); err != nil {
		return err
	}

	if err := checkHiveAudit(c); err != nil {
		return err
	}
	if err := checkApiaryAudit(c, generatedRequestID); err != nil {
		return err
	}

	// 未知范围返回未找到。
	if _, err := c.expect("GET", "/apiaries/apiary-missing/events", nil, 404, `"code":"not_found"`); err != nil {
		return err
	}
	if _, err := c.expect("GET", "/hives/hive-missing/events", nil, 404, `"code":"not_found"`); err != nil {
		return err
	}
	return nil
}

func checkHiveAudit(c *client) error {
	first, err := c.expect("GET", "/hives/"+auditHiveOne+"/events", nil, 200)
	if err != nil {
		return err
	}
	// 蜂箱范围只包含该蜂箱自身的 4 条事实：安装、推进产蜜期、巡检、退役。
	if got := first.count(`"type":`); got != 4 {
		return fmt.Errorf("hive-01 事件数期望 4，实际 %d：%s", got, first.text())
	}
	fragments := []string{
		`"id":"event-00000002"`, `"type":"hive_installed"`, `"requestId":"req-h01-install"`,
		`"model":"langstroth"`, `"newState":"colonized"`,
		`"id":"event-00000003"`, `"type":"hive_state_advanced"`, `"requestId":"req-h01-prod"`,
		`"from":"colonized"`, `"to":"production"`,
		`"id":"event-00000004"`, `"type":"inspection_recorded"`, `"requestId":"req-h01-insp"`,
		`"inspectionId":"inspection-000001"`, `"observedAt":"2026-04-10T09:00:00Z"`,
		`"colonyStrength":3`, `"miteCount":12`, `"mite_load"`, `"temper_risk"`, `"note":"西侧巢门观察"`,
		`"id":"event-00000005"`, `"requestId":"req-h01-retire"`, `"from":"production"`, `"to":"retired"`,
	}
	for _, fragment := range fragments {
		if _, err := c.expect("GET", "/hives/"+auditHiveOne+"/events", nil, 200, fragment); err != nil {
			return err
		}
	}
	for _, pair := range [][2]string{
		{`"event-00000002"`, `"event-00000003"`},
		{`"event-00000003"`, `"event-00000004"`},
		{`"event-00000004"`, `"event-00000005"`},
	} {
		if err := first.mustOrder(pair[0], pair[1]); err != nil {
			return err
		}
	}
	// 蜂场录入与其他蜂箱的事实不出现在蜂箱范围。
	for _, fragment := range []string{
		`"apiary_registered"`, `"queen_replaced"`, `"hive-02"`,
		`2026-05-05T05:05:05Z`, `应被拒绝`, `queen-000003`,
	} {
		if err := first.mustNotContain("GET", "/hives/"+auditHiveOne+"/events", fragment); err != nil {
			return err
		}
	}
	// 重复读取同一范围得到完全一致的内容。
	second, err := c.expect("GET", "/hives/"+auditHiveOne+"/events", nil, 200)
	if err != nil {
		return err
	}
	if first.text() != second.text() {
		return fmt.Errorf("hive-01 事件重复读取内容不一致：\n%s\n%s", first.text(), second.text())
	}

	hiveTwo, err := c.expect("GET", "/hives/"+auditHiveTwo+"/events", nil, 200,
		`"event-00000006"`, `"hive_installed"`,
		`"event-00000007"`, `"newQueenId":"queen-000001"`, `"reason":"aging"`,
		`"event-00000008"`, `"newQueenId":"queen-000002"`, `"previousQueenId":"queen-000001"`,
		`"previousState":"superseded"`, `"reason":"aggression"`, `"origin":"cells"`,
		`"event-00000009"`, `"to":"retired"`,
	)
	if err != nil {
		return err
	}
	if got := hiveTwo.count(`"type":`); got != 4 {
		return fmt.Errorf("hive-02 事件数期望 4，实际 %d：%s", got, hiveTwo.text())
	}
	if err := hiveTwo.mustOrder(`"newQueenId":"queen-000001"`, `"newQueenId":"queen-000002"`); err != nil {
		return err
	}
	for _, fragment := range []string{`"imported"`, `queen-000003`} {
		if err := hiveTwo.mustNotContain("GET", "/hives/"+auditHiveTwo+"/events", fragment); err != nil {
			return err
		}
	}
	return nil
}

func checkApiaryAudit(c *client, generatedRequestID string) error {
	first, err := c.expect("GET", "/apiaries/"+auditApiaryID+"/events", nil, 200)
	if err != nil {
		return err
	}
	// 9 次成功写入：1 次蜂场录入 + hive-01 4 次 + hive-02 4 次；其余被拒绝请求不留事件。
	if got := first.count(`"type":`); got != 9 {
		return fmt.Errorf("蜂场事件数期望 9，实际 %d：%s", got, first.text())
	}
	fragments := []string{
		`"event-00000001"`, `"type":"apiary_registered"`, `"requestId":"req-apiary"`,
		`"name":"北岭蜂场"`, `"district":"北岭片区"`, `"keeper":"周岚"`,
		`"requestId":"req-h01-install"`, `"requestId":"req-h01-prod"`,
		`"requestId":"req-h01-insp"`, `"requestId":"req-h01-retire"`,
		`"requestId":"req-h02-install"`, `"requestId":"req-h02-q1"`, `"requestId":"req-h02-q2"`,
		`"requestId":"` + generatedRequestID + `"`,
	}
	for _, fragment := range fragments {
		if _, err := c.expect("GET", "/apiaries/"+auditApiaryID+"/events", nil, 200, fragment); err != nil {
			return err
		}
	}
	if err := first.mustOrder(`"event-00000001"`, `"event-00000009"`); err != nil {
		return err
	}
	if err := first.mustOrder(`"type":"inspection_recorded"`, `"type":"queen_replaced"`); err != nil {
		return err
	}
	// 被拒绝请求的事实（越界量测、未知状态、非法来源、未采纳的 queen-000003）不得出现。
	for _, fragment := range []string{
		`2026-05-05T05:05:05Z`, `应被拒绝`, `"flying"`, `"imported"`, `queen-000003`,
		`"apiary-bad-header"`,
	} {
		if err := first.mustNotContain("GET", "/apiaries/"+auditApiaryID+"/events", fragment); err != nil {
			return err
		}
	}
	second, err := c.expect("GET", "/apiaries/"+auditApiaryID+"/events", nil, 200)
	if err != nil {
		return err
	}
	if first.text() != second.text() {
		return fmt.Errorf("蜂场事件重复读取内容不一致：\n%s\n%s", first.text(), second.text())
	}
	return nil
}

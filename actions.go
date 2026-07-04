package sdk

func SendTextMessage(accountWxid, targetWxid, content string) Action {
	return Action{
		Type:        ActionSendMessage,
		AccountWxid: accountWxid,
		Payload: map[string]any{
			"target":  targetWxid,
			"type":    "text",
			"content": content,
		},
	}
}

func AddFriend(accountWxid, targetWxid, verifyMessage string) Action {
	return Action{
		Type:        ActionAddFriend,
		AccountWxid: accountWxid,
		Payload: map[string]any{
			"target":        targetWxid,
			"verifyMessage": verifyMessage,
		},
	}
}

func UpdateRemark(accountWxid, targetWxid, remark string) Action {
	return Action{
		Type:        ActionUpdateRemark,
		AccountWxid: accountWxid,
		Payload: map[string]any{
			"target": targetWxid,
			"remark": remark,
		},
	}
}

func SetLabel(accountWxid, targetWxid string, labels []string) Action {
	return Action{
		Type:        ActionSetLabel,
		AccountWxid: accountWxid,
		Payload: map[string]any{
			"target": targetWxid,
			"labels": labels,
		},
	}
}

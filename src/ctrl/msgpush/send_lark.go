package msgpush

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

const (
	appID     = "cli_a327add9e3bdd00c"
	appSecret = "9wC0hCaqp67YykJoaUo9rceRebS7806i"
)

func GetAccessToken() (string, error) {
	url := "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal/"

	body := map[string]string{
		"app_id":     appID,
		"app_secret": appSecret,
	}
	bodyJSON, _ := json.Marshal(body)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(bodyJSON))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if result["code"].(float64) != 0 {
		return "", fmt.Errorf("failed to get access token: %v", result["msg"])
	}

	return result["tenant_access_token"].(string), nil
}

func SendMessage(accessToken, to, content string) error {
	url := "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=open_id"

	body := map[string]interface{}{
		"receive_id": to,
		"content":    fmt.Sprintf("{\"text\":\"%s\"}", content),
		"msg_type":   "text",
	}
	bodyJSON, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(bodyJSON))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("Response:", string(respBody))
	return nil
}

// 根据手机号获取用户 OpenID
func getUserOpenID(accessToken, phone string) (string, error) {
	url := "https://open.feishu.cn/open-apis/contact/v3/users/batch_get_id"

	body := map[string]interface{}{
		"mobiles": []string{phone},
	}
	bodyJSON, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(bodyJSON))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)
	fmt.Println("result ", result)

	if result["code"].(float64) != 0 {
		return "", fmt.Errorf("failed to get user info: %v", result["msg"])
	}

	// 获取用户 OpenID
	data := result["data"].(map[string]interface{})
	userList := data["user_list"].([]interface{})
	if len(userList) == 0 {
		return "", fmt.Errorf("user not found")
	}
	user := userList[0].(map[string]interface{})
	return user["user_id"].(string), nil
}

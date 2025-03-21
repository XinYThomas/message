package consumer

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/BitofferHub/msgcenter/src/config"
	"github.com/BitofferHub/msgcenter/src/constant"
	"github.com/BitofferHub/msgcenter/src/ctrl/ctrlmodel"
	"github.com/BitofferHub/msgcenter/src/ctrl/tools"
	"github.com/BitofferHub/msgcenter/src/data"
	"github.com/BitofferHub/pkg/middlewares/log"
)

type TimerMsgConsume struct {
}

// Consume 方法用于启动消息消费
func (s *TimerMsgConsume) Consume() {
	// 启动处理定时消息
	go s.consumeFromTimer()
}

func (s *TimerMsgConsume) consumeFromTimer() {
	ticker := time.NewTicker(time.Duration(100) * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		s.consumeTimerMsg()
	}
}

func (s *TimerMsgConsume) consumeTimerMsg() {
	// 前后波动500ms
	ctx := context.Background()
	dt := data.GetData()
	now := time.Now().Unix()
	//msgList, err := dt.GetCache().ZRangeByScore(ctx, "Timer_Msgs", "0", strconv.FormatInt(time.Now().UnixMilli(), 10))
	result, err := dt.GetCache().EvalResults(ctx, constant.LUA_ZRANGEBYSCORE_AND_REM, []string{"Timer_Msgs"}, []interface{}{"0", strconv.FormatInt(now, 10)})
	if err != nil {
		return
	}
	timeList, ok := result.([]interface{})
	if !ok || len(timeList) == 0 {
		return
	}

	tmpMsgList, err := data.MsgTmpQueueTimerNsp.GetOnTimeMsgList(dt.GetDB(),
		int(data.TIMER_MSG_STATUS_PENDING), now)

	if err != nil {
		log.ErrorContextf(ctx, "MsgTmpQueueTimerNsp.GetOnTimeMsgList err %s", err.Error())
		return
	}
	// 遍历消息列表，将每个消息的ID添加到msgIdList中
	msgIdList := make([]string, 0)
	for _, dbMsg := range tmpMsgList {
		msgIdList = append(msgIdList, dbMsg.MsgId)
	}

	// 如果msgIdList不为空，则批量设置消息状态为处理中
	if len(msgIdList) != 0 {
		err = data.MsgTmpQueueTimerNsp.BatchSetStatus(dt.GetDB(), msgIdList,
			int(data.TIMER_MSG_STATUS_PROCESSING))
		// 如果批量设置消息状态时发生错误，则返回
		if err != nil {
			return
		}
	}

	// 遍历消息列表，处理每个消息
	for _, dbMsg := range tmpMsgList {
		var req = new(ctrlmodel.SendMsgReq)
		// 反序列化消息
		err = json.Unmarshal([]byte(dbMsg.Req), &req)
		if err != nil {
			log.ErrorContextf(ctx, "unmarshal message err %s", err.Error())
			return
		}
		// 处理消息
		req.MsgID = dbMsg.MsgId
		err = reSendOneMsg(ctx, req)
		if err != nil {
			// 重试一次消息
			err = reSendOneMsg(ctx, req)
			if err != nil {
				log.ErrorContextf(ctx, "dealOneMsg err %s", err.Error())
				return
			}
		}
	}

	// 如果msgIdList不为空，则批量设置消息状态为处理中
	if len(msgIdList) != 0 {
		err = data.MsgTmpQueueTimerNsp.BatchSetStatus(dt.GetDB(), msgIdList,
			int(data.TIMER_MSG_STATUS_SUCC))
		// 如果批量设置消息状态时发生错误，则返回
		if err != nil {
			return
		}
	}
}

// dealOneMsg 处理一条消息
func reSendOneMsg(ctx context.Context, req *ctrlmodel.SendMsgReq) error {
	dt := data.GetData()

	// 获取消息模板
	tp, err := dt.GetMsgTemplate(ctx, req.TemplateID)
	if err != nil {
		log.ErrorContextf(ctx, "获取消息模板失败: %s", err.Error())
	}

	var sendErr error
	if config.Conf.Common.MySQLAsMq {
		sendErr = sendToMySQL(ctx, req)
		if sendErr != nil {
			log.Errorf(" timer sendToMySQL err %s", sendErr.Error())
		}
	} else {
		sendErr = sendToMQ(ctx, req)
		if sendErr != nil {
			log.Errorf(" timer sendToMQ err %s", sendErr.Error())
		}
	}

	// 不管发送是否成功，都更新消息记录状态
	// 如果发送成功，标记为"待处理"；如果发送失败，标记为失败或错误状态
	var status int
	if sendErr != nil {
		status = int(data.MSG_STATUS_FAILED) // 使用负数表示错误状态
	} else {
		status = int(data.MSG_STATUS_PENDING) // 发送成功，标记为待处理
	}

	// 使用通用函数更新消息记录状态
	updateErr := tools.CreateOrUpdateMsgRecord(dt.GetDB(), req.MsgID, req, tp, status)
	if updateErr != nil {
		log.ErrorContextf(ctx, "更新定时消息记录状态失败: %s", updateErr.Error())
		// 更新状态失败不影响主流程
	}

	// 如果发送失败，返回错误
	if sendErr != nil {
		return sendErr
	}

	return nil
}

func sendToMySQL(ctx context.Context, req *ctrlmodel.SendMsgReq) error {
	// 获取数据实例
	dt := data.GetData()

	// 创建一个新的消息队列实例
	var md = new(data.MsgQueue)

	// 设置消息的主题
	md.Subject = req.Subject

	// 设置消息的模板ID
	md.TemplateID = req.TemplateID

	// 将模板数据转换为JSON格式
	td, err := json.Marshal(req.TemplateData)
	if err != nil {
		return err
	}

	// 设置消息的模板数据
	md.TemplateData = string(td)

	// 设置消息的接收者
	md.To = req.To

	// 设置消息的ID
	md.MsgId = req.MsgID

	// 设置消息的初始状态
	md.Status = int(data.TASK_STATUS_PENDING)
	md.Priority = req.Priority

	// 将消息插入到MySQL数据库中
	err = data.MsgQueueNsp.Create(dt.GetDB(),
		data.GetPriorityStr(data.PriorityEnum(req.Priority)), md)
	if err != nil {
		return err
	}
	// 返回nil，表示发送成功
	return nil
}

func sendToMQ(ctx context.Context, req *ctrlmodel.SendMsgReq) error {
	// 获取数据实例
	log.Infof("into sendToMQ")
	dt := data.GetData()

	// 将请求结构体转换为JSON格式
	msgJson, err := json.Marshal(req)
	if err != nil {
		log.ErrorContextf(context.Background(), "json marshal err %s", err.Error())
		return err
	}

	// 根据消息优先级选择对应的消息队列生产者
	if req.Priority == int(data.PRIORITY_LOW) {
		// 获取低优先级消息队列生产者
		producer := dt.GetLowMQProducer()
		// 发送消息到低优先级消息队列
		return producer.SendMessage(msgJson)
	} else if req.Priority == int(data.PRIORITY_MIDDLE) {
		// 获取中优先级消息队列生产者
		producer := dt.GetMiddleMQProducer()
		// 发送消息到中优先级消息队列
		return producer.SendMessage(msgJson)
	} else if req.Priority == int(data.PRIORITY_HIGH) {
		// 获取高优先级消息队列生产者
		producer := dt.GetHighMQProducer()
		// 发送消息到高优先级消息队列
		return producer.SendMessage(msgJson)
	}
	// 返回nil，表示发送成功
	return nil
}

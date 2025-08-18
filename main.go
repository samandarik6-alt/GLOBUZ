package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Konstantalar
const (
	BOT_TOKEN         = "8214075520:AAH2NuC9spv7D8up4dFJnW8PariCeSrf0aM"
	REMINDER_DELAY    = 10 * time.Minute
	PENDING_JSON_FILE = "pending_messages.json"
	GROUPS_JSON_FILE  = "groups.json"
	ADMIN_CHAT_ID     = -1002816907697 // Adminlar guruhi - bu yerdan xabar olmaydi
)

// Strukturalar
type PendingMessage struct {
	MessageID      int       `json:"message_id"`
	GroupID        int64     `json:"group_id"`
	GroupTitle     string    `json:"group_title"`
	UserID         int64     `json:"user_id"`
	Username       string    `json:"username"`
	Text           string    `json:"text"`
	Timestamp      time.Time `json:"timestamp"`
	LastReminder   time.Time `json:"last_reminder"`
	ReminderCount  int       `json:"reminder_count"`
	Status         string    `json:"status"` // "pending", "answered", "ignored"
	AnsweredBy     int64     `json:"answered_by,omitempty"`
	AnsweredAt     time.Time `json:"answered_at,omitempty"`
	SentMessageIDs []int     `json:"sent_message_ids,omitempty"` // Yuborilgan eslatma xabarlar ID lari
}

type GroupInfo struct {
	GroupID     int64     `json:"group_id"`
	GroupTitle  string    `json:"group_title"`
	GroupType   string    `json:"group_type"`
	JoinedAt    time.Time `json:"joined_at"`
	IsActive    bool      `json:"is_active"`
	AdminIDs    []int64   `json:"admin_ids"`
	LastUpdated time.Time `json:"last_updated"`
}

type PendingMessagesData struct {
	Messages map[string]*PendingMessage `json:"messages"`
}

type GroupsData struct {
	Groups map[string]*GroupInfo `json:"groups"`
}

// Topic ma'lumotlari
type TopicInfo struct {
	ChatID          int64  `json:"chat_id"`
	MessageThreadID int    `json:"message_thread_id"`
	Text            string `json:"text"`
}

// Global o'zgaruvchilar
var (
	bot             *tgbotapi.BotAPI
	pendingMessages = make(map[int]*PendingMessage)
	monitoredGroups = make(map[int64]*GroupInfo)
	topicsList      = []TopicInfo{}
)

// JSON fayldan pending messages yuklash
func loadPendingMessages() {
	if _, err := os.Stat(PENDING_JSON_FILE); os.IsNotExist(err) {
		log.Printf("📁 Pending messages fayli mavjud emas: %s", PENDING_JSON_FILE)
		return
	}

	data, err := ioutil.ReadFile(PENDING_JSON_FILE)
	if err != nil {
		log.Printf("❌ Pending messages faylni o'qishda xato: %v", err)
		return
	}

	var pendingData PendingMessagesData
	err = json.Unmarshal(data, &pendingData)
	if err != nil {
		log.Printf("❌ Pending messages JSON parse qilishda xato: %v", err)
		return
	}

	for msgIDStr, msg := range pendingData.Messages {
		msgID, _ := strconv.Atoi(msgIDStr)
		pendingMessages[msgID] = msg
	}

	log.Printf("✅ %d ta javobsiz xabar yuklandi", len(pendingMessages))
}

// JSON faylga pending messages saqlash
func savePendingMessages() {
	pendingData := PendingMessagesData{
		Messages: make(map[string]*PendingMessage),
	}

	for msgID, msg := range pendingMessages {
		pendingData.Messages[strconv.Itoa(msgID)] = msg
	}

	data, err := json.MarshalIndent(pendingData, "", "  ")
	if err != nil {
		log.Printf("❌ Pending messages JSON yaratishda xato: %v", err)
		return
	}

	err = ioutil.WriteFile(PENDING_JSON_FILE, data, 0644)
	if err != nil {
		log.Printf("❌ Pending messages faylga yozishda xato: %v", err)
		return
	}
}

// JSON fayldan guruhlar ma'lumotini yuklash
func loadGroups() {
	if _, err := os.Stat(GROUPS_JSON_FILE); os.IsNotExist(err) {
		log.Printf("📁 Guruhlar fayli mavjud emas: %s", GROUPS_JSON_FILE)
		return
	}

	data, err := ioutil.ReadFile(GROUPS_JSON_FILE)
	if err != nil {
		log.Printf("❌ Guruhlar faylni o'qishda xato: %v", err)
		return
	}

	var groupsData GroupsData
	err = json.Unmarshal(data, &groupsData)
	if err != nil {
		log.Printf("❌ Guruhlar JSON parse qilishda xato: %v", err)
		return
	}

	for groupIDStr, group := range groupsData.Groups {
		groupID, _ := strconv.ParseInt(groupIDStr, 10, 64)
		monitoredGroups[groupID] = group
	}

	log.Printf("✅ %d ta guruh ma'lumoti yuklandi", len(monitoredGroups))
}

// JSON faylga guruhlar ma'lumotini saqlash
func saveGroups() {
	groupsData := GroupsData{
		Groups: make(map[string]*GroupInfo),
	}

	for groupID, group := range monitoredGroups {
		groupsData.Groups[strconv.FormatInt(groupID, 10)] = group
	}

	data, err := json.MarshalIndent(groupsData, "", "  ")
	if err != nil {
		log.Printf("❌ Guruhlar JSON yaratishda xato: %v", err)
		return
	}

	err = ioutil.WriteFile(GROUPS_JSON_FILE, data, 0644)
	if err != nil {
		log.Printf("❌ Guruhlar faylga yozishda xato: %v", err)
		return
	}

	log.Printf("💾 %d ta guruh ma'lumoti saqlandi", len(monitoredGroups))
}

// Guruhga qo'shilganda yoki guruh ma'lumotini yangilash
func updateGroupInfo(chat *tgbotapi.Chat) {
	if chat.Type != "group" && chat.Type != "supergroup" {
		return
	}

	groupInfo, exists := monitoredGroups[chat.ID]
	if !exists {
		groupInfo = &GroupInfo{
			GroupID:    chat.ID,
			GroupTitle: chat.Title,
			GroupType:  chat.Type,
			JoinedAt:   time.Now(),
			IsActive:   true,
			AdminIDs:   []int64{},
		}
		monitoredGroups[chat.ID] = groupInfo
		log.Printf("🆕 Yangi guruh qo'shildi: %s (ID: %d)", chat.Title, chat.ID)
	}

	// Guruh ma'lumotlarini yangilash
	groupInfo.GroupTitle = chat.Title
	groupInfo.GroupType = chat.Type
	groupInfo.LastUpdated = time.Now()
	groupInfo.IsActive = true

	// Guruh adminlarini olish
	go updateGroupAdmins(chat.ID)

	saveGroups()
}

// Guruh adminlarini yangilash
func updateGroupAdmins(groupID int64) {
	adminConfig := tgbotapi.ChatAdministratorsConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: groupID,
		},
	}

	admins, err := bot.GetChatAdministrators(adminConfig)
	if err != nil {
		log.Printf("❌ Guruh adminlarini olishda xato (%d): %v", groupID, err)
		return
	}

	if groupInfo, exists := monitoredGroups[groupID]; exists {
		groupInfo.AdminIDs = []int64{}
		for _, admin := range admins {
			groupInfo.AdminIDs = append(groupInfo.AdminIDs, admin.User.ID)
		}
		log.Printf("👥 Guruh %d da %d ta admin topildi", groupID, len(groupInfo.AdminIDs))
		saveGroups()
	}
}

// Admin ekanligini tekshirish (username orqali)
func isAdminMessage(username string, groupID int64) bool {
	// Username bo'lmasa false
	if username == "" {
		return false
	}

	// Username da "globuz" so'zi borligini tekshirish
	if strings.Contains(strings.ToLower(username), "globuz") {
		return true
	}

	return false
}

// Eslatmalarni tekshirish va yuborish
func checkAndSendReminders() {
	now := time.Now()
	log.Printf("🔍 Eslatmalar tekshirilmoqda... Jami pending: %d", len(pendingMessages))

	for msgID, pendingMsg := range pendingMessages {
		if pendingMsg.Status != "pending" {
			continue // Faqat javobsiz xabarlar uchun
		}

		timeSinceMessage := now.Sub(pendingMsg.Timestamp)
		timeSinceLastReminder := now.Sub(pendingMsg.LastReminder)

		shouldSendReminder := false

		if pendingMsg.ReminderCount == 0 && timeSinceMessage >= REMINDER_DELAY {
			shouldSendReminder = true
			log.Printf("⏰ Birinchi eslatma vaqti keldi: MSG %d (%v o'tdi)", msgID, timeSinceMessage)
		} else if pendingMsg.ReminderCount > 0 && timeSinceLastReminder >= REMINDER_DELAY {
			shouldSendReminder = true
			log.Printf("⏰ Keyingi eslatma vaqti keldi: MSG %d (%v o'tdi)", msgID, timeSinceLastReminder)
		}

		if shouldSendReminder {
			log.Printf("📤 Eslatma yuborilmoqda: MSG %d", msgID)
			sendAdminReminder(pendingMsg)
			pendingMsg.LastReminder = now
			pendingMsg.ReminderCount++
			savePendingMessages()
		}
	}
}

// Guruh nomidan davlat nomlarini aniqlash
func extractCountriesFromGroupTitle(groupTitle string) []string {
	var countries []string

	// | belgisi bilan bo'lingan qismlarni tekshirish
	parts := strings.Split(groupTitle, "|")
	for _, part := range parts {
		part = strings.TrimSpace(part)

		// Har bir qismni topiclar bilan solishtirish
		for _, topic := range topicsList {
			if strings.EqualFold(part, topic.Text) {
				countries = append(countries, topic.Text)
			}
		}

		// # belgisidan keyin davlat nomlarini ham qidirish
		if strings.HasPrefix(part, "#") {
			country := strings.TrimPrefix(part, "#")
			country = strings.TrimSpace(country)

			// Topiclar bilan solishtirish
			for _, topic := range topicsList {
				if strings.EqualFold(country, topic.Text) {
					countries = append(countries, topic.Text)
				}
			}
		}
	}

	return countries
}

// Xabar matnidan davlat nomini topish
func findCountryInText(text string) string {
	// Avval xabar matnidan qidirish
	for _, topic := range topicsList {
		if strings.Contains(strings.ToLower(text), strings.ToLower(topic.Text)) {
			return topic.Text
		}
	}

	return ""
}

// Guruh nomidan davlat nomini topish
func findCountryFromGroupTitle(groupTitle string) string {
	countries := extractCountriesFromGroupTitle(groupTitle)
	if len(countries) > 0 {
		return countries[0]
	}
	return ""
}

// Davlat nomi bo'yicha topic ID topish
func findTopicByCountry(country string) *TopicInfo {
	for _, topic := range topicsList {
		if strings.EqualFold(topic.Text, country) {
			return &topic
		}
	}
	return nil
}

// Adminlarga eslatma yuborish - TO'G'RILANGAN VERSIYA
func sendAdminReminder(pendingMsg *PendingMessage) {
	log.Printf("🔔 Adminlarga eslatma yuborilmoqda: MSG %d", pendingMsg.MessageID)

	// Avval xabar matnidan davlat nomini topish
	country := findCountryInText(pendingMsg.Text)

	// Agar xabar matnidan topa olmasa, guruh nomidan qidirish
	if country == "" {
		country = findCountryFromGroupTitle(pendingMsg.GroupTitle)
	}

	// Agar hali ham topilmasa
	if country == "" {
		country = "Aniqlanmadi"
	}

	// Topic topish
	topic := findTopicByCountry(country)
	var targetChatID int64 = pendingMsg.GroupID
	var targetThreadID int = 0

	if topic != nil {
		targetChatID = topic.ChatID
		targetThreadID = topic.MessageThreadID
		log.Printf("🎯 Topic topildi: %s -> Chat: %d, Thread: %d", country, targetChatID, targetThreadID)
	} else {
		log.Printf("❌ Topic topilmadi davlat uchun: %s, asl guruhga yuboriladi", country)
	}

	reminderText := fmt.Sprintf(`⚠️ JAVOBSIZ XABAR! (%d-ESLATMA)

🏢 Guruh: %s
📍 Davlat: %s
👤 Foydalanuvchi: @%s (ID: %d)
⏰ Xabar vaqti: %s
📝 Xabar matni: "%s"

🔔 %s dan beri javob kutmoqda!
⏱️ Jami eslatmalar: %d

Iltimos tezroq javob bering!`,
		pendingMsg.ReminderCount+1,
		pendingMsg.GroupTitle,
		country,
		pendingMsg.Username,
		pendingMsg.UserID,
		pendingMsg.Timestamp.Format("02.01.2006 15:04:05"),
		pendingMsg.Text,
		formatDuration(time.Since(pendingMsg.Timestamp)),
		pendingMsg.ReminderCount)

	// Topicga xabar yuborish
	msg := tgbotapi.NewMessage(targetChatID, reminderText)

	// Forum topic thread ID ni qo'shish
	if targetThreadID > 0 {
		// Qo'lda MessageThreadID ni o'rnatish
		msgWithThread := tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:           targetChatID,
				ReplyToMessageID: targetThreadID,
			},
			Text: reminderText,
		}
		msg = msgWithThread
	}

	// "Javob berildi" va "Xabarni ko'rish" tugmalarini qo'shish
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Javob berildi", fmt.Sprintf("mark_answered_%d", pendingMsg.MessageID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("📄 Xabarni ko'rish", fmt.Sprintf("https://t.me/c/%d/%d", -pendingMsg.GroupID-1000000000000, pendingMsg.MessageID)),
		),
	)
	msg.ReplyMarkup = keyboard

	sentMsg, err := bot.Send(msg)
	if err != nil {
		log.Printf("❌ Topicga eslatma yuborishda xato: %v", err)
	} else {
		// Yuborilgan xabar ID sini SentMessageIDs ga qo'shish
		pendingMsg.SentMessageIDs = append(pendingMsg.SentMessageIDs, sentMsg.MessageID)
		log.Printf("✅ Topic %d ga eslatma yuborildi (MSG ID: %d)", targetThreadID, sentMsg.MessageID)
	}

	log.Printf("🎯 Eslatma yuborish tugallandi: MSG %d", pendingMsg.MessageID)
}

// Vaqt formatini chiroyli ko'rsatish
func formatDuration(d time.Duration) string {
	seconds := int(d.Seconds())
	if seconds < 60 {
		return fmt.Sprintf("%d soniya", seconds)
	}
	minutes := seconds / 60
	remainingSeconds := seconds % 60
	if minutes < 60 {
		return fmt.Sprintf("%d daqiqa %d soniya", minutes, remainingSeconds)
	}
	hours := minutes / 60
	remainingMinutes := minutes % 60
	return fmt.Sprintf("%d soat %d daqiqa", hours, remainingMinutes)
}

// JSON fayldan topiclar ma'lumotini yuklash
func loadTopics() {
	// Hardcoded topics ma'lumotlari
	topicsList = []TopicInfo{
		{ChatID: -1002816907697, MessageThreadID: 2, Text: "UK"},
		{ChatID: -1002816907697, MessageThreadID: 4, Text: "Schengen"},
		{ChatID: -1002816907697, MessageThreadID: 6, Text: "Australia"},
		{ChatID: -1002816907697, MessageThreadID: 8, Text: "Japan"},
		{ChatID: -1002816907697, MessageThreadID: 10, Text: "Peru"},
		{ChatID: -1002816907697, MessageThreadID: 12, Text: "India"},
		{ChatID: -1002816907697, MessageThreadID: 14, Text: "Argentina"},
		{ChatID: -1002816907697, MessageThreadID: 16, Text: "Uganda"},
		{ChatID: -1002816907697, MessageThreadID: 18, Text: "Kuwait"},
		{ChatID: -1002816907697, MessageThreadID: 20, Text: "Pakistan"},
		{ChatID: -1002816907697, MessageThreadID: 22, Text: "Albania"},
		{ChatID: -1002816907697, MessageThreadID: 24, Text: "Hong Kong"},
		{ChatID: -1002816907697, MessageThreadID: 26, Text: "Ireland"},
		{ChatID: -1002816907697, MessageThreadID: 28, Text: "Cyprus"},
		{ChatID: -1002816907697, MessageThreadID: 30, Text: "Zimbabwe"},
	}

	log.Printf("✅ %d ta topic yuklandi", len(topicsList))
}

// Asosiy main funksiya
func main() {
	var err error
	bot, err = tgbotapi.NewBotAPI(BOT_TOKEN)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true
	log.Printf("🚀 Bot %s ga ulanildi", bot.Self.UserName)

	// JSON fayllardan ma'lumotlarni yuklash
	loadPendingMessages()
	loadGroups()
	loadTopics()

	log.Printf("📊 Kuzatilayotgan guruhlar: %d ta", len(monitoredGroups))
	log.Printf("📋 Mavjud topiclar: %d ta", len(topicsList))

	// Har 30 soniyada eslatmalarni tekshirish (test uchun)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		log.Printf("⏰ Eslatma timer boshlandi (har 30 soniyada)")

		for {
			select {
			case <-ticker.C:
				checkAndSendReminders()
			}
		}
	}()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			handleMessage(update.Message)
		} else if update.CallbackQuery != nil {
			handleCallbackQuery(update.CallbackQuery)
		} else if update.MyChatMember != nil {
			handleChatMemberUpdate(update.MyChatMember)
		}
	}
}

// Bot guruhga qo'shilganda yoki chiqarilganda
func handleChatMemberUpdate(chatMember *tgbotapi.ChatMemberUpdated) {
	if chatMember.NewChatMember.User.ID == bot.Self.ID {
		if chatMember.NewChatMember.Status == "administrator" || chatMember.NewChatMember.Status == "member" {
			log.Printf("🎉 Bot guruhga qo'shildi: %s (ID: %d)", chatMember.Chat.Title, chatMember.Chat.ID)
			updateGroupInfo(&chatMember.Chat)
		} else if chatMember.NewChatMember.Status == "left" || chatMember.NewChatMember.Status == "kicked" {
			log.Printf("👋 Bot guruhdan chiqarildi: %s (ID: %d)", chatMember.Chat.Title, chatMember.Chat.ID)
			if groupInfo, exists := monitoredGroups[chatMember.Chat.ID]; exists {
				groupInfo.IsActive = false
				saveGroups()
			}
		}
	}
}

// Xabarlarni boshqarish
func handleMessage(message *tgbotapi.Message) {
	// Guruh xabarlarini tekshirish
	if message.Chat.Type == "group" || message.Chat.Type == "supergroup" {
		// Guruh ma'lumotlarini yangilash
		updateGroupInfo(message.Chat)
		handleGroupMessage(message)
		return
	}

	// Private xabarlarni ignore qilish
	log.Printf("📝 Private xabar e'tiborga olinmadi: %s dan", message.From.FirstName)
}

// Guruh xabarlarini boshqarish
func handleGroupMessage(message *tgbotapi.Message) {
	if message.From.ID == bot.Self.ID {
		return
	}

	groupID := message.Chat.ID

	// Admin guruhidan xabar olmaydi
	if groupID == ADMIN_CHAT_ID {
		log.Printf("🚫 Admin guruhidan xabar e'tiborga olinmadi: %s", message.Chat.Title)
		return
	}

	username := message.From.UserName

	log.Printf("📨 Guruh xabari: %s (@%s) dan %s guruhida (Admin: %v)",
		message.From.FirstName, username, message.Chat.Title, isAdminMessage(username, groupID))

	// Admin javobini tekshirish
	if isAdminMessage(username, groupID) {
		if message.ReplyToMessage != nil {
			// Bot tomonidan yuborilgan xabarga javob berilganligini tekshirish
			if message.ReplyToMessage.From.ID == bot.Self.ID {
				// Bot xabarining ID si orqali pending message topish
				replyToMessageID := message.ReplyToMessage.MessageID

				for _, pendingMsg := range pendingMessages {
					// Agar bot yuborgan xabar ID si mavjud bo'lsa
					for _, sentMsgID := range pendingMsg.SentMessageIDs {
						if sentMsgID == replyToMessageID {
							pendingMsg.Status = "answered"
							pendingMsg.AnsweredBy = message.From.ID
							pendingMsg.AnsweredAt = time.Now()

							log.Printf("✅ Admin bot xabariga javob berdi: Pending MSG %d", pendingMsg.MessageID)

							// Barcha yuborilgan eslatma xabarlarini o'chirish
							deleteSentMessages(pendingMsg)

							savePendingMessages()
							return
						}
					}
				}
			}

			// Eski usul - oddiy reply
			originalMessageID := message.ReplyToMessage.MessageID
			if pendingMsg, exists := pendingMessages[originalMessageID]; exists {
				pendingMsg.Status = "answered"
				pendingMsg.AnsweredBy = message.From.ID
				pendingMsg.AnsweredAt = time.Now()

				log.Printf("✅ Admin javob berdi: %d xabarga guruh %d da", originalMessageID, groupID)

				// Barcha eslatma xabarlarini o'chirish
				deleteSentMessages(pendingMsg)

				savePendingMessages()
			}
		}
		return
	}

	// Oddiy userlarning xabarlarini kuzatish
	if username == "" {
		username = message.From.FirstName
	}

	groupInfo := monitoredGroups[groupID]
	groupTitle := "Noma'lum guruh"
	if groupInfo != nil {
		groupTitle = groupInfo.GroupTitle
	}

	pendingMsg := &PendingMessage{
		MessageID:      message.MessageID,
		GroupID:        groupID,
		GroupTitle:     groupTitle,
		UserID:         message.From.ID,
		Username:       username,
		Text:           message.Text,
		Timestamp:      time.Now(),
		LastReminder:   time.Time{},
		ReminderCount:  0,
		Status:         "pending",
		SentMessageIDs: []int{}, // Yuborilgan eslatma xabar ID lari uchun slice
	}

	pendingMessages[message.MessageID] = pendingMsg
	savePendingMessages()

	log.Printf("🔔 Yangi user xabari saqlandi: MSG %d, %s dan %s guruhida", message.MessageID, username, groupTitle)
}

// Callback query boshqarish
func handleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	userID := callback.From.ID
	data := callback.Data

	bot.Send(tgbotapi.NewCallback(callback.ID, ""))

	// Faqat "mark_answered" va "show_message" callback larni boshqarish
	if strings.HasPrefix(data, "mark_answered_") {
		msgIDStr := strings.TrimPrefix(data, "mark_answered_")
		msgID, err := strconv.Atoi(msgIDStr)
		if err == nil {
			if pendingMsg, exists := pendingMessages[msgID]; exists {
				pendingMsg.Status = "answered"
				pendingMsg.AnsweredBy = userID
				pendingMsg.AnsweredAt = time.Now()

				log.Printf("✅ Admin tomonidan javob berildi deb belgilandi: %d xabar", msgID)

				// Barcha yuborilgan eslatma xabarlarini o'chirish
				deleteSentMessages(pendingMsg)

				// Callback javobini yuborish
				bot.Send(tgbotapi.NewCallbackWithAlert(callback.ID, "✅ Xabar javob berildi deb belgilandi va barcha adminlardan o'chirildi!"))

				savePendingMessages()
			}
		}
	} else if strings.HasPrefix(data, "show_message_") {
		// show_message_GROUPID_MESSAGEID formatini parse qilish
		parts := strings.Split(data, "_")
		if len(parts) == 4 {
			groupIDStr := parts[2]
			msgIDStr := parts[3]

			groupID, err1 := strconv.ParseInt(groupIDStr, 10, 64)
			msgID, err2 := strconv.Atoi(msgIDStr)

			if err1 == nil && err2 == nil {
				// Message linkini yaratish
				messageLink := fmt.Sprintf("https://t.me/c/%d/%d", -groupID-1000000000000, msgID)

				if pendingMsg, exists := pendingMessages[msgID]; exists {
					// Faqat xabar matni va link tugmasi
					keyboard := tgbotapi.NewInlineKeyboardMarkup(
						tgbotapi.NewInlineKeyboardRow(
							tgbotapi.NewInlineKeyboardButtonURL("Xabarni ochish", messageLink),
						),
					)

					msg := tgbotapi.NewMessage(callback.Message.Chat.ID, pendingMsg.Text)
					msg.ReplyMarkup = keyboard
					bot.Send(msg)
					bot.Send(tgbotapi.NewCallback(callback.ID, ""))
				} else {
					bot.Send(tgbotapi.NewCallback(callback.ID, "Xabar topilmadi"))
				}
			}
		}
	}
}

// Barcha yuborilgan eslatma xabarlarini o'chirish
func deleteSentMessages(pendingMsg *PendingMessage) {
	if len(pendingMsg.SentMessageIDs) == 0 {
		return
	}

	deletedCount := 0
	for _, messageID := range pendingMsg.SentMessageIDs {
		deleteMsg := tgbotapi.NewDeleteMessage(ADMIN_CHAT_ID, messageID)
		_, err := bot.Request(deleteMsg)
		if err != nil {
			log.Printf("❌ Admin guruhdan xabar %d ni o'chirishda xato: %v", messageID, err)
		} else {
			deletedCount++
			log.Printf("🗑️ Admin guruhdan eslatma xabari o'chirildi (MSG ID: %d)", messageID)
		}
	}

	log.Printf("🗑️ Jami %d ta eslatma xabari o'chirildi", deletedCount)

	// SentMessageIDs ni tozalash
	pendingMsg.SentMessageIDs = []int{}
}

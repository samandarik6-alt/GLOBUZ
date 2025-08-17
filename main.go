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
	REMINDER_DELAY    = 5 * time.Second
	PENDING_JSON_FILE = "pending_messages.json"
	GROUPS_JSON_FILE  = "groups.json"
	TOPICS_JSON_FILE  = "topics.json"
	ADMIN_CHAT_ID     = -1002816907697 // Adminlar guruhi - bu yerdan xabar olmaydi
)

// Strukturalar
type VisaInfo struct {
	Flag           string `json:"flag"`
	ServicePrice   string `json:"service_price"`
	VisaType       string `json:"visa_type"`
	VisaFee        string `json:"visa_fee"`
	ProcessingTime string `json:"processing_time"`
	Requirements   string `json:"requirements"`
	Details        string `json:"details"`
}

type UserSession struct {
	Step          int    `json:"step"`
	SelectedVisa  string `json:"selected_visa"`
	Name          string `json:"name"`
	TravelHistory string `json:"travel_history"`
	WorkInfo      string `json:"work_info"`
	BankInfo      string `json:"bank_info"`
	FamilyInfo    string `json:"family_info"`
	Phone         string `json:"phone"`
	UserID        int64  `json:"user_id"`
	Username      string `json:"username"`
}

type PendingMessage struct {
	MessageID          int           `json:"message_id"`
	GroupID            int64         `json:"group_id"`
	GroupTitle         string        `json:"group_title"`
	UserID             int64         `json:"user_id"`
	Username           string        `json:"username"`
	Text               string        `json:"text"`
	Timestamp          time.Time     `json:"timestamp"`
	LastReminder       time.Time     `json:"last_reminder"`
	ReminderCount      int           `json:"reminder_count"`
	Status             string        `json:"status"` // "pending", "answered", "ignored"
	AnsweredBy         int64         `json:"answered_by,omitempty"`
	AnsweredAt         time.Time     `json:"answered_at,omitempty"`
	ReminderMessageIDs map[int64]int `json:"reminder_message_ids,omitempty"` // adminID -> messageID
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

type TopicsData struct {
	Topics []TopicInfo `json:"topics"`
}

// Global o'zgaruvchilar
var (
	bot             *tgbotapi.BotAPI
	sessions        = make(map[int64]*UserSession)
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

	// ReminderMessageIDs ni initialize qilish
	if pendingMsg.ReminderMessageIDs == nil {
		pendingMsg.ReminderMessageIDs = make(map[int64]int)
	}

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

	// "Javob berildi" tugmasini qo'shish
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Javob berildi", fmt.Sprintf("mark_answered_%d", pendingMsg.MessageID)),
		),
	)
	msg.ReplyMarkup = keyboard

	sentMsg, err := bot.Send(msg)
	if err != nil {
		log.Printf("❌ Topicga eslatma yuborishda xato: %v", err)
	} else {
		// Topic message ID ni saqlash
		pendingMsg.ReminderMessageIDs[targetChatID] = sentMsg.MessageID
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
	userID := message.From.ID

	// Guruh xabarlarini tekshirish
	if message.Chat.Type == "group" || message.Chat.Type == "supergroup" {
		// Guruh ma'lumotlarini yangilash
		updateGroupInfo(message.Chat)
		handleGroupMessage(message)
		return
	}

	if message.IsCommand() {
		switch message.Command() {
		case "start":
			handleStart(userID, message.From.FirstName)
		case "groups":
			if isAdminMessage(message.From.UserName, 0) {
				showGroupsList(userID)
			}
		case "stats":
			if isAdminMessage(message.From.UserName, 0) {
				showStats(userID)
			}
		case "test":
			// Test komandasi - adminlarga test xabari yuborish
			if isAdminMessage(message.From.UserName, 0) {
				testMessage := &PendingMessage{
					MessageID:          999999,
					GroupID:            message.Chat.ID,
					GroupTitle:         "Test Group",
					UserID:             userID,
					Username:           message.From.UserName,
					Text:               "Bu test xabari",
					Timestamp:          time.Now(),
					LastReminder:       time.Time{},
					ReminderCount:      0,
					Status:             "pending",
					ReminderMessageIDs: make(map[int64]int),
				}
				sendAdminReminder(testMessage)
				msg := tgbotapi.NewMessage(userID, "✅ Test eslatma yuborildi!")
				bot.Send(msg)
			}
		}
		return
	}

	session, exists := sessions[userID]
	if !exists {
		return
	}

	switch session.Step {
	case 1:
		session.Name = message.Text
		session.Step = 2
		askForTravelHistory(userID)
	case 2:
		session.TravelHistory = message.Text
		session.Step = 3
		askForWorkInfo(userID)
	case 3:
		session.WorkInfo = message.Text
		session.Step = 4
		askForBankInfo(userID)
	case 4:
		session.BankInfo = message.Text
		session.Step = 5
		askForFamilyInfo(userID)
	case 5:
		session.FamilyInfo = message.Text
		session.Step = 6
		askForPhone(userID)
	case 6:
		session.Phone = message.Text
		session.Username = message.From.UserName
		submitApplication(session)
		confirmApplication(userID, session)
		delete(sessions, userID)
	}
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
					for chatID, messageID := range pendingMsg.ReminderMessageIDs {
						if chatID == groupID && messageID == replyToMessageID {
							pendingMsg.Status = "answered"
							pendingMsg.AnsweredBy = message.From.ID
							pendingMsg.AnsweredAt = time.Now()

							log.Printf("✅ Admin bot xabariga javob berdi: Pending MSG %d", pendingMsg.MessageID)

							// Barcha eslatma xabarlarini o'chirish
							deleteReminderMessages(pendingMsg)

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

				// Eslatma xabarlarini o'chirish
				deleteReminderMessages(pendingMsg)

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
		MessageID:          message.MessageID,
		GroupID:            groupID,
		GroupTitle:         groupTitle,
		UserID:             message.From.ID,
		Username:           username,
		Text:               message.Text,
		Timestamp:          time.Now(),
		LastReminder:       time.Time{},
		ReminderCount:      0,
		Status:             "pending",
		ReminderMessageIDs: make(map[int64]int),
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

	switch {
	case data == "visa_service":
		showCountrySelection(userID)
	case data == "prices":
		showPricesMenu(userID)
	case data == "countries_list":
		showAllCountries(userID)
	case data == "contact":
		showContact(userID)
	case data == "top_visas":
		showTopVisas(userID)
	case data == "all_countries":
		showAllCountries(userID)
	case data == "cheap_visas":
		showCheapVisas(userID)
	case data == "premium_visas":
		showPremiumVisas(userID)
	case data == "back_main":
		handleStart(userID, callback.From.FirstName)
	case data == "back_prices":
		showPricesMenu(userID)
	case data == "back_countries":
		showCountrySelection(userID)
	case strings.HasPrefix(data, "country_"):
		country := strings.TrimPrefix(data, "country_")
		showCountryDetails(userID, country)
	case strings.HasPrefix(data, "apply_"):
		country := strings.TrimPrefix(data, "apply_")
		startApplication(userID, country, callback.From.UserName)
	case data == "enter_name":
		askForName(userID)
	case data == "enter_travel":
		askForTravelHistory(userID)
	case data == "enter_work":
		askForWorkInfo(userID)
	case data == "enter_bank":
		askForBankInfo(userID)
	case data == "enter_family":
		askForFamilyInfo(userID)
	case data == "enter_phone":
		askForPhone(userID)
	case data == "new_application":
		showCountrySelection(userID)
	case data == "operator":
		showContact(userID)
	case strings.HasPrefix(data, "mark_answered_"):
		msgIDStr := strings.TrimPrefix(data, "mark_answered_")
		msgID, err := strconv.Atoi(msgIDStr)
		if err == nil {
			if pendingMsg, exists := pendingMessages[msgID]; exists {
				pendingMsg.Status = "answered"
				pendingMsg.AnsweredBy = userID
				pendingMsg.AnsweredAt = time.Now()

				log.Printf("✅ Admin tomonidan javob berildi deb belgilandi: %d xabar", msgID)

				// Barcha adminlardan eslatma xabarlarini o'chirish
				deleteReminderMessages(pendingMsg)

				// Callback javobini yuborish
				bot.Send(tgbotapi.NewCallbackWithAlert(callback.ID, "✅ Xabar javob berildi deb belgilandi va barcha adminlardan o'chirildi!"))

				savePendingMessages()
			}
		}
	}
}

// Start komandasi
func handleStart(userID int64, firstName string) {
	text := fmt.Sprintf(`Assalomu alaykum %s! 👋

🏢 GLOBUZ VISA AGENTLIGI
Men sizning shaxsiy visa yordamchingizman.

5 yildan ortiq tajriba bilan 50+ davlatga visa olishda yordam beramiz!

Sizga qanday yordam bera olishim mumkin?`, firstName)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏢 Viza xizmati kerak", "visa_service"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 Narxlarni ko'rish", "prices"),
			tgbotapi.NewInlineKeyboardButtonData("📋 Davlatlar ro'yxati", "countries_list"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📞 Bog'lanish", "contact"),
		),
	)

	msg := tgbotapi.NewMessage(userID, text)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// Narxlar menyusi
func showPricesMenu(userID int64) {
	text := `💰 VISA NARXLARI BO'LIMI

Qaysi kategoriyani ko'rishni xohlaysiz?`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔝 TOP vizalar", "top_visas"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🌍 Barcha davlatlar", "all_countries"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 Arzon vizalar", "cheap_visas"),
			tgbotapi.NewInlineKeyboardButtonData("🏆 Premium vizalar", "premium_visas"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Asosiy menyu", "back_main"),
		),
	)

	msg := tgbotapi.NewMessage(userID, text)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// TOP vizalar
func showTopVisas(userID int64) {
	text := `🔝 TOP VIZALAR

🇺🇸 USA
💼 Xizmat: 300$
📋 Visa to'lovi: 185 USD
⏰ Muddati: Turlicha

🇪🇺 Schengen
💼 Xizmat: 300$
📋 Visa to'lovi: 90 euro + appointment to'lovi
⏰ Muddati: 15-45 kun

🇬🇧 UK
💼 Xizmat: 500$
📋 Visa to'lovi: 280 USD + 179 USD + 77 GBP
⏰ Muddati: Turlicha`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📞 Konsultatsiya olish", "contact"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏢 Ariza berish", "visa_service"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Narxlar menyusi", "back_prices"),
		),
	)

	msg := tgbotapi.NewMessage(userID, text)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// Davlat tanlash
func showCountrySelection(userID int64) {
	text := `🏢 VIZA XIZMATI

Qaysi davlat vizasini olmoqchisiz?

🔥 Mashhur yo'nalishlar:
• 🇺🇸 Amerika - 300$ (1 yillik)
• 🇪🇺 Schengen - 300$ (Evropa)
• 🇬🇧 UK - 500$ (Britaniya)

Yoki avval narxlarni ko'ring 👇`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇺🇸 Amerika", "country_USA"),
			tgbotapi.NewInlineKeyboardButtonData("🇪🇺 Schengen", "country_Schengen"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇬🇧 UK", "country_UK"),
			tgbotapi.NewInlineKeyboardButtonData("🇨🇦 Kanada", "country_Canada"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇦🇺 Avstraliya", "country_Australia"),
			tgbotapi.NewInlineKeyboardButtonData("🇯🇵 Yaponiya", "country_Japan"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇧🇷 Braziliya", "country_Brazil"),
			tgbotapi.NewInlineKeyboardButtonData("🇸🇦 Saudiya", "country_Saudi Arabia"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 Narxlarni ko'rish", "prices"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Boshqa davlatlar", "all_countries"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Asosiy menyu", "back_main"),
		),
	)

	msg := tgbotapi.NewMessage(userID, text)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// Barcha davlatlar
func showAllCountries(userID int64) {
	text := `🌍 BARCHA DAVLATLAR

Quyidagi davlatlar uchun visa xizmatini taklif etamiz:`

	keyboard := tgbotapi.NewInlineKeyboardMarkup()

	countries := [][]string{
		{"USA", "Schengen", "UK"},
		{"Canada", "Australia", "New Zealand"},
		{"Japan", "Brazil", "Saudi Arabia"},
		{"India", "South Africa", "Seychelles"},
		{"Uganda", "Kuwait", "Bahrain"},
		{"Israel", "Pakistan", "Vietnam"},
		{"Albania", "Taiwan", "Turkey"},
		{"UAE", "Qatar", "Oman"},
		{"Jordan", "Egypt", "Morocco"},
		{"Tunisia", "Kenya", "Tanzania"},
		{"Ethiopia"},
	}

	for _, row := range countries {
		var buttons []tgbotapi.InlineKeyboardButton
		for _, country := range row {
			if visa, exists := visaData[country]; exists {
				buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(
					visa.Flag+" "+country, "country_"+country))
			}
		}
		if len(buttons) > 0 {
			keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, buttons)
		}
	}

	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Orqaga", "back_countries"),
		),
	)

	msg := tgbotapi.NewMessage(userID, text)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// Arzon vizalar
func showCheapVisas(userID int64) {
	text := `💰 ARZON VIZALAR

🇸🇦 Saudi Arabia - BEPUL xizmat
🇮🇳 India - 100$ xizmat
🇵🇰 Pakistan - 100$ xizmat (visa bepul)
🇻🇳 Vietnam - 100$ xizmat
🇸🇨 Seychelles - 100$ xizmat
🇰🇼 Kuwait - 100$ xizmat
🇧🇭 Bahrain - 100$ xizmat
🇹🇷 Turkey - 100$ xizmat`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇸🇦 Saudi Arabia", "country_Saudi Arabia"),
			tgbotapi.NewInlineKeyboardButtonData("🇮🇳 India", "country_India"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇵🇰 Pakistan", "country_Pakistan"),
			tgbotapi.NewInlineKeyboardButtonData("🇻🇳 Vietnam", "country_Vietnam"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇸🇨 Seychelles", "country_Seychelles"),
			tgbotapi.NewInlineKeyboardButtonData("🇰🇼 Kuwait", "country_Kuwait"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇧🇭 Bahrain", "country_Bahrain"),
			tgbotapi.NewInlineKeyboardButtonData("🇹🇷 Turkey", "country_Turkey"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Narxlar menyusi", "back_prices"),
		),
	)

	msg := tgbotapi.NewMessage(userID, text)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// Premium vizalar
func showPremiumVisas(userID int64) {
	text := `🏆 PREMIUM VIZALAR

🇨🇦 Canada - 700$ xizmat
🇦🇺 Australia - 700$ xizmat
🇳🇿 New Zealand - 700$ xizmat
🇬🇧 UK - 500$ xizmat
🇿🇦 South Africa - 500$ xizmat
🇮🇱 Israel - 500$ xizmat`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇨🇦 Canada", "country_Canada"),
			tgbotapi.NewInlineKeyboardButtonData("🇦🇺 Australia", "country_Australia"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇳🇿 New Zealand", "country_New Zealand"),
			tgbotapi.NewInlineKeyboardButtonData("🇬🇧 UK", "country_UK"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇿🇦 South Africa", "country_South Africa"),
			tgbotapi.NewInlineKeyboardButtonData("🇮🇱 Israel", "country_Israel"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Narxlar menyusi", "back_prices"),
		),
	)

	msg := tgbotapi.NewMessage(userID, text)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// Davlat tafsilotlari
func showCountryDetails(userID int64, country string) {
	visa, exists := visaData[country]
	if !exists {
		return
	}

	text := fmt.Sprintf(`%s %s VIZASI

💼 Bizning xizmat: %s
📋 Visa to'lovi: %s
⏰ Jarayon vaqti: %s

🎯 Visa turi: %s

📄 Talablar: %s

ℹ️ Qo'shimcha: %s

Ariza berishni xohlaysizmi?`,
		visa.Flag, country, visa.ServicePrice, visa.VisaFee,
		visa.ProcessingTime, visa.VisaType, visa.Requirements, visa.Details)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Ariza berish", "apply_"+country),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 Boshqa narxlar", "prices"),
			tgbotapi.NewInlineKeyboardButtonData("📞 Konsultatsiya", "contact"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Davlat tanlash", "back_countries"),
		),
	)

	msg := tgbotapi.NewMessage(userID, text)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// Ariza berish boshlash
func startApplication(userID int64, country, username string) {
	visa := visaData[country]

	text := fmt.Sprintf(`🏢 %s VIZASI UCHUN ARIZA

%s Tanlangan davlat: %s
💼 Xizmat narxi: %s

Ariza berish uchun bir necha savolga javob bering.

❓ 1-savol: Ismingizni bilsam bo'ladimi?`,
		country, visa.Flag, country, visa.ServicePrice)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📝 Ha, ismimni yozaman", "enter_name"),
		),
	)

	sessions[userID] = &UserSession{
		Step:         0,
		SelectedVisa: country,
		UserID:       userID,
		Username:     username,
	}

	msg := tgbotapi.NewMessage(userID, text)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// Ism so'rash
func askForName(userID int64) {
	text := `🪪 Iltimos, to'liq ismingizni yozing:

(Masalan: Aliyev Vali Akramovich)`

	sessions[userID].Step = 1

	msg := tgbotapi.NewMessage(userID, text)
	bot.Send(msg)
}

// Sayohat tarixi so'rash
func askForTravelHistory(userID int64) {
	session, exists := sessions[userID]
	if !exists {
		return
	}

	text := fmt.Sprintf(`Rahmat %s! 👋

🌍 SAYOHAT TARIXI

Avval qaysi davlatlarga borganmisiz?

Masalan:
• Turkiya - 2022 yilda 7 kun
• Dubay - 2023 yilda 5 kun
• Agar hech qayerga bormagan bo'lsangiz "Hech qayerga bormaganman" deb yozing`, session.Name)

	session.Step = 2

	msg := tgbotapi.NewMessage(userID, text)
	bot.Send(msg)
}

// Ish joyi ma'lumoti so'rash
func askForWorkInfo(userID int64) {
	text := `💼 ISH JOYI MA'LUMOTI

Hozirgi ish joyingiz haqida ma'lumot bering:

Masalan:
• Kompaniya nomi: "IT Solutions"
• Lavozim: Dasturchi
• Maosh: 8 million som
• Agar ishlamasangiz "Ishlamayman" deb yozing`

	session := sessions[userID]
	session.Step = 3

	msg := tgbotapi.NewMessage(userID, text)
	bot.Send(msg)
}

// Bank ma'lumoti so'rash
func askForBankInfo(userID int64) {
	text := `💰 MOLIYAVIY HOLAT

Bank hisobingiz va daromadingiz haqida ma'lumot bering:

Masalan:
• Bank hisobida: 25 million som
• Qo'shimcha daromad: Freelance ish
• Agar kam bo'lsa ham haqiqiy summani yozing`

	session := sessions[userID]
	session.Step = 4

	msg := tgbotapi.NewMessage(userID, text)
	bot.Send(msg)
}

// Oila ma'lumoti so'rash
func askForFamilyInfo(userID int64) {
	text := `🏠 OILA AHVOLI

Oila ahvolingiz haqida ma'lumot bering:

Masalan:
• Oilaliman, 1 ta farzandim bor
• Yolg'izman
• Ota-onam bilan yashayман
• Turmush qurmaganman`

	session := sessions[userID]
	session.Step = 5

	msg := tgbotapi.NewMessage(userID, text)
	bot.Send(msg)
}

// Telefon raqam so'rash
func askForPhone(userID int64) {
	text := fmt.Sprintf(`Juda yaxshi %s! ✅

📞 Menejerlarimiz siz bilan bog'lanishi uchun telefon raqamingizni qoldirishingizni so'raymiz.

30 daqiqa ichida mutaxassis siz bilan bog'lanadi!

Telefon raqamingizni yozing (masalan: +998901234567):`, sessions[userID].Name)

	session := sessions[userID]
	session.Step = 6

	msg := tgbotapi.NewMessage(userID, text)
	bot.Send(msg)
}

// Arizani yuborish
func submitApplication(session *UserSession) {
	visa := visaData[session.SelectedVisa]
	currentTime := time.Now().Format("02.01.2006 15:04")

	username := session.Username
	if username == "" {
		username = "mavjud emas"
	}

	groupMessage := fmt.Sprintf(`🆕 YANGI ARIZA - %s

👤 MIJOZ MA'LUMOTLARI:
🪪 F.I.O: %s
📞 Telefon: %s
📱 Telegram: @%s
🆔 User ID: %d

🌍 VISA MA'LUMOTLARI:
📍 Davlat: %s %s
🎯 Visa turi: %s
💼 Xizmat narxi: %s
💳 Visa to'lovi: %s
⏰ Jarayon vaqti: %s
📋 Talablar: %s

📝 BATAFSIL MA'LUMOT:

🌍 SAYOHAT TARIXI:
%s

💼 ISH JOYI:
%s

💰 MOLIYAVIY HOLAT:
%s

🏠 OILA AHVOLI:
%s

⏰ ARIZA VAQTI: %s
━━━━━━━━━━━━━━━━━━━━━━

🔥 OPERATOR! 30 DAQIQA ICHIDA MIJOZ BILAN BOG'LANING!`,
		session.SelectedVisa,
		session.Name,
		session.Phone,
		username,
		session.UserID,
		session.SelectedVisa,
		visa.Flag,
		visa.VisaType,
		visa.ServicePrice,
		visa.VisaFee,
		visa.ProcessingTime,
		visa.Requirements,
		session.TravelHistory,
		session.WorkInfo,
		session.BankInfo,
		session.FamilyInfo,
		currentTime)

	// Faqat admin guruhiga yuborish (ADMIN_CHAT_ID ga)
	msg := tgbotapi.NewMessage(ADMIN_CHAT_ID, groupMessage)
	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("❌ Admin guruhiga ariza yuborishda xato: %v", err)
	} else {
		log.Printf("✅ Admin guruhiga ariza yuborildi: %s", session.Name)
	}
}

// Arizani tasdiqlash
func confirmApplication(userID int64, session *UserSession) {
	visa := visaData[session.SelectedVisa]

	text := fmt.Sprintf(`✅ ARIZA MUVAFFAQIYATLI YUBORILDI!

Hurmatli %s, sizning %s vizasi uchun arizangiz qabul qilindi!

📋 TANLANGAN XIZMAT:
%s %s
💼 Xizmat: %s
💳 Visa to'lovi: %s
⏰ Jarayon: %s

🔥 30 DAQIQA ICHIDA MENEJERIMIZ SIZ BILAN BOG'LANADI!

📱 Telegram: @globuz_support
☎️ Telefon: +998 90 123 45 67

Bizni tanlaganingiz uchun rahmat! 🙏`,
		session.Name,
		session.SelectedVisa,
		visa.Flag,
		session.SelectedVisa,
		visa.ServicePrice,
		visa.VisaFee,
		visa.ProcessingTime)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🆕 Yangi ariza", "new_application"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 Narxlar", "prices"),
			tgbotapi.NewInlineKeyboardButtonData("📞 Operator", "operator"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Asosiy menyu", "back_main"),
		),
	)

	msg := tgbotapi.NewMessage(userID, text)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// Bog'lanish ma'lumotlari
func showContact(userID int64) {
	text := `📞 OPERATOR BILAN BOG'LANISH

🚀 Tez yordam olish uchun:

📱 Telegram: @globuz_support
☎️ Telefon: +998 90 123 45 67
☎️ Qo'shimcha: +998 95 123 45 67
📧 Email: info@globuzvisa.uz

⏰ Ish vaqti:
9:00-18:00 (Dush-Juma)
10:00-16:00 (Shanba)

🔥 Operator 15 daqiqa ichida javob beradi!
📋 Arizangiz tafsilotlarini tayyorlab qo'ying 👇`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🆕 Yangi ariza", "new_application"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 Narxlar", "prices"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Asosiy menyu", "back_main"),
		),
	)

	msg := tgbotapi.NewMessage(userID, text)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// Adminlarga guruhlar ro'yxatini ko'rsatish
func showGroupsList(userID int64) {
	if len(monitoredGroups) == 0 {
		msg := tgbotapi.NewMessage(userID, "📋 Hozircha kuzatilayotgan guruhlar yo'q.")
		bot.Send(msg)
		return
	}

	text := "📋 KUZATILAYOTGAN GURUHLAR:\n\n"

	for _, group := range monitoredGroups {
		status := "🔴 Faol emas"
		if group.IsActive {
			status = "🟢 Faol"
		}

		text += fmt.Sprintf("🏢 %s\n📊 ID: %d\n%s\n👥 Adminlar: %d ta\n⏰ Qo'shilgan: %s\n\n",
			group.GroupTitle,
			group.GroupID,
			status,
			len(group.AdminIDs),
			group.JoinedAt.Format("02.01.2006 15:04"))
	}

	msg := tgbotapi.NewMessage(userID, text)
	bot.Send(msg)
}

// Statistikani ko'rsatish
func showStats(userID int64) {
	totalPending := 0
	totalAnswered := 0

	for _, msg := range pendingMessages {
		if msg.Status == "pending" {
			totalPending++
		} else if msg.Status == "answered" {
			totalAnswered++
		}
	}

	text := fmt.Sprintf(`📊 BOT STATISTIKASI

🏢 Kuzatilayotgan guruhlar: %d ta
🔔 Javobsiz xabarlar: %d ta
✅ Javob berilgan: %d ta
📝 Jami xabarlar: %d ta

👨‍💼 Global adminlar: %d ta`,
		len(monitoredGroups),
		totalPending,
		totalAnswered,
		len(pendingMessages))

	msg := tgbotapi.NewMessage(userID, text)
	bot.Send(msg)
}

// Barcha adminlardan eslatma xabarlarini o'chirish - TO'G'RILANGAN VERSIYA
func deleteReminderMessages(pendingMsg *PendingMessage) {
	if pendingMsg.ReminderMessageIDs == nil {
		return
	}

	deletedCount := 0
	for chatID, messageID := range pendingMsg.ReminderMessageIDs {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
		_, err := bot.Request(deleteMsg)
		if err != nil {
			log.Printf("❌ Chat %d dan xabar %d ni o'chirishda xato: %v", chatID, messageID, err)
		} else {
			deletedCount++
			log.Printf("🗑️ Chat %d dan eslatma xabari o'chirildi (MSG ID: %d)", chatID, messageID)
		}
	}

	log.Printf("🗑️ Jami %d ta eslatma xabari o'chirildi", deletedCount)

	// ReminderMessageIDs ni tozalash
	pendingMsg.ReminderMessageIDs = make(map[int64]int)
}

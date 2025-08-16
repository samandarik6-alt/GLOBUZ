package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Konstantalar
const (
	BOT_TOKEN         = "8214075520:AAH2NuC9spv7D8up4dFJnW8PariCeSrf0aM"
	REMINDER_DELAY    = 10 * time.Second
	PENDING_JSON_FILE = "pending_messages.json"
	GROUPS_JSON_FILE  = "groups.json"
	CHAT_CONFIG_FILE  = "chat_config.json" // Added chat config file constant
)

// Global adminlar ro'yxati - username asosida
var ADMIN_USERNAMES = []string{
	"globuz",
	"globuz_admin",
	"globuz_super",
	"admin_globuz",
}

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
	MessageID     int       `json:"message_id"`
	GroupID       int64     `json:"group_id"`
	GroupTitle    string    `json:"group_title"`
	UserID        int64     `json:"user_id"`
	Username      string    `json:"username"`
	Text          string    `json:"text"`
	Timestamp     time.Time `json:"timestamp"`
	LastReminder  time.Time `json:"last_reminder"`
	ReminderCount int       `json:"reminder_count"`
	Status        string    `json:"status"` // "pending", "answered", "ignored"
	AnsweredBy    int64     `json:"answered_by,omitempty"`
	AnsweredAt    time.Time `json:"answered_at,omitempty"`
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

type ChatConfig struct {
	ChatID          int64  `json:"chat_id"`
	MessageThreadID int    `json:"message_thread_id"`
	Text            string `json:"text"`
}

// Global o'zgaruvchilar
var (
	bot             *tgbotapi.BotAPI
	sessions        = make(map[int64]*UserSession)
	pendingMessages []*PendingMessage
	monitoredGroups = make(map[int64]*GroupInfo)
	chatConfigs     []ChatConfig
)

var pendingMessagesMutex sync.Mutex

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

	for _, msg := range pendingData.Messages {
		pendingMessages = append(pendingMessages, msg)
	}

	log.Printf("✅ %d ta javobsiz xabar yuklandi", len(pendingMessages))
}

// JSON faylga pending messages saqlash
func savePendingMessages() {
	pendingData := PendingMessagesData{
		Messages: make(map[string]*PendingMessage),
	}

	for _, msg := range pendingMessages {
		pendingData.Messages[strconv.Itoa(msg.MessageID)] = msg
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

// Admin ekanligini tekshirish (username asosida)
func isAdminMessage(username string, groupID int64) bool {
	// Username asosida admin tekshirish - "globuz" yoki "GLOBUZ" bo'lsa admin
	if username != "" {
		lowerUsername := strings.ToLower(username)
		if strings.Contains(lowerUsername, "globuz") {
			return true
		}
	}
	return false
}

// Eslatmalarni tekshirish va yuborish
func checkAndSendReminders() {
	now := time.Now()

	pendingMessagesMutex.Lock()
	defer pendingMessagesMutex.Unlock()

	for _, pendingMsg := range pendingMessages {
		if pendingMsg.Status != "pending" {
			continue // Faqat javobsiz xabarlar uchun
		}

		timeSinceMessage := now.Sub(pendingMsg.Timestamp)
		timeSinceLastReminder := now.Sub(pendingMsg.LastReminder)

		shouldSendReminder := false

		if pendingMsg.ReminderCount == 0 && timeSinceMessage >= REMINDER_DELAY {
			shouldSendReminder = true
		} else if pendingMsg.ReminderCount > 0 && timeSinceLastReminder >= REMINDER_DELAY {
			shouldSendReminder = true
		}

		if shouldSendReminder {
			sendAdminReminder(pendingMsg)
			pendingMsg.LastReminder = now
			pendingMsg.ReminderCount++
			savePendingMessages()
		}
	}
}

// Adminlarga eslatma yuborish - guruh topiciga
func sendAdminReminder(pendingMsg *PendingMessage) {
	// Find matching chat config
	matchingConfig := findMatchingChatConfig(pendingMsg.GroupTitle, pendingMsg.Text)

	if matchingConfig == nil {
		log.Printf("❌ %s guruh yoki '%s' matn uchun mos chat config topilmadi", pendingMsg.GroupTitle, pendingMsg.Text)
		return
	}

	messageLink := fmt.Sprintf("https://t.me/c/%d/%d",
		-pendingMsg.GroupID-1000000000000, // Convert original group ID to positive number for link
		pendingMsg.MessageID)

	reminderText := fmt.Sprintf(`⚠️ JAVOBSIZ XABAR! (%d-ESLATMA)

🏢 Guruh: %s
🆔 Guruh ID: %d
👤 Foydalanuvchi: @%s (ID: %d)
⏰ Xabar vaqti: %s
📝 Xabar matni: "%s"

🔔 %s dan beri javob kutmoqda!
⏱️ Jami eslatmalar: %d

Iltimos tezroq javob bering!
%s`,
		pendingMsg.ReminderCount+1,
		pendingMsg.GroupTitle,
		pendingMsg.GroupID,
		pendingMsg.Username,
		pendingMsg.UserID,
		pendingMsg.Timestamp.Format("02.01.2006 15:04:05"),
		pendingMsg.Text,
		formatDuration(time.Since(pendingMsg.Timestamp)),
		pendingMsg.ReminderCount,
		messageLink)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("📝 Javob berish", messageLink),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Javob berildi", fmt.Sprintf("answered_%d_%d", pendingMsg.GroupID, pendingMsg.MessageID)),
		),
	)

	err := sendMessageWithThread(matchingConfig.ChatID, matchingConfig.MessageThreadID, reminderText, &keyboard)
	if err != nil {
		log.Printf("❌ Guruh topiciga (%d, thread: %d) eslatma yuborishda xato: %v",
			matchingConfig.ChatID, matchingConfig.MessageThreadID, err)
	} else {
		log.Printf("✅ Eslatma yuborildi: Guruh %d, Topic %d",
			matchingConfig.ChatID, matchingConfig.MessageThreadID)
	}
}

// Vaqt formatini chiroyli ko'rsatish
func formatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	if minutes < 60 {
		return fmt.Sprintf("%d daqiqa", minutes)
	}
	hours := minutes / 60
	remainingMinutes := minutes % 60
	return fmt.Sprintf("%d soat %d daqiqa", hours, remainingMinutes)
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
	log.Printf("👨‍💼 Global admin usernames: %v", ADMIN_USERNAMES)

	// JSON fayllardan ma'lumotlarni yuklash
	loadPendingMessages()
	loadGroups()
	loadChatConfigs() // Load chat configurations

	log.Printf("📊 Kuzatilayotgan guruhlar: %d ta", len(monitoredGroups))
	for groupID, group := range monitoredGroups {
		log.Printf("  - %s (ID: %d, Adminlar: %d ta)", group.GroupTitle, groupID, len(group.AdminIDs))
	}

	log.Printf("📋 Chat konfiguratsiyalar: %d ta", len(chatConfigs))
	for _, config := range chatConfigs {
		log.Printf("  - %s -> Chat ID: %d, Topic ID: %d", config.Text, config.ChatID, config.MessageThreadID)
	}

	// Har daqiqada eslatmalarni tekshirish
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

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
	username := message.From.UserName

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
			if isAdminMessage(username, 0) {
				showGroupsList(userID)
			}
		case "stats":
			if isAdminMessage(username, 0) {
				showStats(userID)
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
		session.Username = username
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

	if message.Chat.ID == -1002816907697 {
		return
	}

	groupID := message.Chat.ID
	userID := message.From.ID
	username := message.From.UserName

	// Admin javobini tekshirish
	if isAdminMessage(username, groupID) {
		if message.ReplyToMessage != nil {
			originalMessageID := message.ReplyToMessage.MessageID
			pendingMessagesMutex.Lock()
			defer pendingMessagesMutex.Unlock()
			for i, msg := range pendingMessages {
				if msg.MessageID == originalMessageID && msg.GroupID == groupID {
					msg.Status = "answered"
					msg.AnsweredBy = userID
					msg.AnsweredAt = time.Now()
					pendingMessages = append(pendingMessages[:i], pendingMessages[i+1:]...)
					log.Printf("✅ Admin javob berdi: %d xabarga guruh %d da", originalMessageID, groupID)
					savePendingMessages()
					break
				}
			}
		}
		return
	}

	// Oddiy userlarning xabarlarini kuzatish
	groupInfo := monitoredGroups[groupID]
	groupTitle := "Noma'lum guruh"
	if groupInfo != nil {
		groupTitle = groupInfo.GroupTitle
	}

	pendingMsg := &PendingMessage{
		MessageID:     message.MessageID,
		GroupID:       groupID,
		GroupTitle:    groupTitle,
		UserID:        userID,
		Username:      username,
		Text:          message.Text,
		Timestamp:     time.Now(),
		LastReminder:  time.Time{},
		ReminderCount: 0,
		Status:        "pending",
	}

	pendingMessagesMutex.Lock()
	pendingMessages = append(pendingMessages, pendingMsg)
	pendingMessagesMutex.Unlock()

	log.Printf("🔔 Yangi user xabari: %s dan %s guruhida", username, groupTitle)
}

// Callback query boshqarish
func handleCallbackQuery(callbackQuery *tgbotapi.CallbackQuery) {
	userID := callbackQuery.From.ID
	data := callbackQuery.Data

	bot.Send(tgbotapi.NewCallback(callbackQuery.ID, ""))

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
		handleStart(userID, callbackQuery.From.FirstName)
	case data == "back_prices":
		showPricesMenu(userID)
	case data == "back_countries":
		showCountrySelection(userID)
	case strings.HasPrefix(data, "country_"):
		country := strings.TrimPrefix(data, "country_")
		showCountryDetails(userID, country)
	case strings.HasPrefix(data, "apply_"):
		country := strings.TrimPrefix(data, "apply_")
		startApplication(userID, country, callbackQuery.From.UserName)
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
			pendingMessagesMutex.Lock()
			defer pendingMessagesMutex.Unlock()
			for i, msg := range pendingMessages {
				if msg.MessageID == msgID {
					msg.Status = "answered"
					msg.AnsweredBy = userID
					msg.AnsweredAt = time.Now()
					pendingMessages = append(pendingMessages[:i], pendingMessages[i+1:]...)
					log.Printf("✅ Admin tomonidan javob berildi deb belgilandi: %d xabar", msgID)
					savePendingMessages()
					break
				}
			}
			// Callback javobini yuborish
			bot.Send(tgbotapi.NewCallbackWithAlert(callbackQuery.ID, "✅ Xabar javob berildi deb belgilandi!"))
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
			if visa, exists := VisaData[country]; exists {
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
	visa, exists := VisaData[country]
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
	visa := VisaData[country]

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
• Ota-onam bilan yashayman
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
	visa := VisaData[session.SelectedVisa]
	currentTime := time.Now().Format("02.01.2006 15:04")

	username := session.Username
	if username == "" {
		username = "mavjud emas"
	}

	// Barcha faol guruhlarga yuborish
	for _, group := range monitoredGroups {
		if !group.IsActive {
			continue
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

		msg := tgbotapi.NewMessage(group.GroupID, groupMessage)
		_, err := bot.Send(msg)
		if err != nil {
			log.Printf("❌ Guruh %s ga ariza yuborishda xato: %v", group.GroupTitle, err)
		} else {
			log.Printf("✅ Guruh %s ga ariza yuborildi: %s", group.GroupTitle, session.Name)
		}
	}
}

// Arizani tasdiqlash
func confirmApplication(userID int64, session *UserSession) {
	visa := VisaData[session.SelectedVisa]

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

	pendingMessagesMutex.Lock()
	defer pendingMessagesMutex.Unlock()

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
		len(pendingMessages),
		len(ADMIN_USERNAMES))

	msg := tgbotapi.NewMessage(userID, text)
	bot.Send(msg)
}

func loadChatConfigs() {
	data, err := ioutil.ReadFile(CHAT_CONFIG_FILE)
	if err != nil {
		log.Printf("❌ Chat config faylini o'qishda xato: %v", err)
		return
	}

	err = json.Unmarshal(data, &chatConfigs)
	if err != nil {
		log.Printf("❌ Chat config JSON parse qilishda xato: %v", err)
		return
	}

	log.Printf("✅ %d ta chat config yuklandi", len(chatConfigs))
}

func findMatchingChatConfig(groupTitle, messageText string) *ChatConfig {
	// First try to match with group title
	for _, config := range chatConfigs {
		if strings.Contains(strings.ToLower(groupTitle), strings.ToLower(config.Text)) {
			return &config
		}
	}

	// Then try to match with message content
	for _, config := range chatConfigs {
		if strings.Contains(strings.ToLower(messageText), strings.ToLower(config.Text)) {
			return &config
		}
	}

	return nil
}

func sendMessageWithThread(chatID int64, threadID int, text string, keyboard *tgbotapi.InlineKeyboardMarkup) error {
	// Prepare the request payload
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}

	if threadID != 0 {
		payload["message_thread_id"] = threadID
	}

	if keyboard != nil {
		payload["reply_markup"] = keyboard
	}

	// Convert payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("JSON marshal error: %v", err)
	}

	// Send HTTP request to Telegram API
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", bot.Token)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("HTTP request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s", string(body))
	}

	return nil
}

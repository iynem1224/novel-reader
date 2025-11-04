package lang

import (
	"fmt"
	"sync"
)

type Locale string

const (
	LocaleEnglish Locale = "en"
	LocaleChinese Locale = "zh"
)

type TabsStrings struct {
	History   string
	Library   string
	Discovery string
	Settings  string
}

type SettingsStrings struct {
	LanguageLabel           string
	LanguageDetail          string
	LineSpacingLabel        string
	LineSpacingDetail       string
	AddFolderLabel          string
	AddFolderDetail         string
	RemoveFolderLabel       string
	RemoveFolderDetail      string
	RemoveOnlineLabel       string
	RemoveOnlineDetail      string
	CountSuffixNone         string
	CountSuffixSingle       string
	CountSuffixMultiple     string
	LanguageNames           map[Locale]string
	LineSpacingUpdateFailed string
	FolderDialogUnavailable string
	SaveConfigFailed        string
}

type SearchStrings struct {
	Placeholder          string
	Prompt               string
	InputHint            string
	FoundTemplate        string
	LoadingTitleTemplate string
	LoadingGeneric       string
	Searching            string
	ErrorTemplate        string
	FailedTemplate       string
	SearchFailedTemplate string
}

type ConfirmStrings struct {
	RemoveFolderPromptTemplate string
	RemoveFolderConfirm        string
	RemoveOnlinePromptTemplate string
	RemoveOnlineConfirm        string
}

type BookshelfStrings struct {
	DiscoveryName string
	UnknownType   string
}

type ReaderStrings struct {
	LoadingDefault       string
	LoadingTitleTemplate string
	ChapterTemplate      string
}

type TOCStrings struct {
	Title          string
	StatusSingular string
	StatusPlural   string
	FilterPrompt   string
}

type DialogStrings struct {
	SelectFolderPrompt string
}

type NovelStrings struct {
	LocalPrefix string
}

type CommonStrings struct {
	UnknownState string
}

type LayoutStrings struct {
	UnderlineLength int
}

type Strings struct {
	Tabs      TabsStrings
	Settings  SettingsStrings
	Search    SearchStrings
	Confirm   ConfirmStrings
	Bookshelf BookshelfStrings
	Reader    ReaderStrings
	TOC       TOCStrings
	Dialog    DialogStrings
	Novel     NovelStrings
	Common    CommonStrings
	Layout    LayoutStrings
}

var (
	mu sync.RWMutex

	translations = map[Locale]*Strings{
		LocaleChinese: {
			Tabs: TabsStrings{
				History:   "历史",
				Library:   "书架",
				Discovery: "发现",
				Settings:  "设置",
			},
			Settings: SettingsStrings{
				LanguageLabel:       "语言",
				LanguageDetail:      "使用左右键切换语言",
				LineSpacingLabel:    "行间距",
				LineSpacingDetail:   "使用左右键调整行间距",
				AddFolderLabel:      "导入本地文件夹",
				AddFolderDetail:     "导入一个本地小说文件夹",
				RemoveFolderLabel:   "删除本地文件夹",
				RemoveFolderDetail:  "删除一个本地小说文件夹",
				RemoveOnlineLabel:   "删除网络小说缓存",
				RemoveOnlineDetail:  "删除一个已缓存的网络小说",
				CountSuffixNone:     "(无)",
				CountSuffixSingle:   "(1)",
				CountSuffixMultiple: "(%d)",
				LanguageNames: map[Locale]string{
					LocaleChinese: "中文",
					LocaleEnglish: "英文",
				},
				LineSpacingUpdateFailed: "无法更新行间距: %v",
				FolderDialogUnavailable: "系统不支持文件夹选择对话框。",
				SaveConfigFailed:        "无法保存设置: %v",
			},
			Search: SearchStrings{
				Placeholder:          "输入小说名称..",
				Prompt:               "搜索：",
				InputHint:            "输入关键词后按 Enter 搜索",
				FoundTemplate:        "找到%d本书籍",
				LoadingTitleTemplate: "正在加载「%s」…",
				LoadingGeneric:       "正在加载小说…",
				Searching:            "搜索中…",
				ErrorTemplate:        "错误: %v",
				FailedTemplate:       "加载「%s」失败: %v",
				SearchFailedTemplate: "搜索失败: %v",
			},
			Confirm: ConfirmStrings{
				RemoveFolderPromptTemplate: "确认要删除 %s?",
				RemoveFolderConfirm:        "确认",
				RemoveOnlinePromptTemplate: "确认要删除缓存 《%s》?",
				RemoveOnlineConfirm:        "确认",
			},
			Bookshelf: BookshelfStrings{
				DiscoveryName: "发现",
				UnknownType:   "未知类型",
			},
			Reader: ReaderStrings{
				LoadingDefault:       "章节加载中…",
				LoadingTitleTemplate: "正在加载「%s」…",
				ChapterTemplate:      "第%d章",
			},
			TOC: TOCStrings{
				Title:          "目录",
				StatusSingular: "章",
				StatusPlural:   "章",
				FilterPrompt:   "搜索：",
			},
			Dialog: DialogStrings{
				SelectFolderPrompt: "选择小说文件夹",
			},
			Novel: NovelStrings{
				LocalPrefix: "本地",
			},
			Common: CommonStrings{
				UnknownState: "未知状态",
			},
			Layout: LayoutStrings{
				UnderlineLength: 48,
			},
		},
		LocaleEnglish: {
			Tabs: TabsStrings{
				History:   "History",
				Library:   "Library",
				Discovery: "Discover",
				Settings:  "Settings",
			},
			Settings: SettingsStrings{
				LanguageLabel:       "Language",
				LanguageDetail:      "Use left/right to switch language",
				LineSpacingLabel:    "Line Spacing",
				LineSpacingDetail:   "Use left/right to adjust line spacing",
				AddFolderLabel:      "Add Local Folder",
				AddFolderDetail:     "Import a local novel folder",
				RemoveFolderLabel:   "Remove Local Folder",
				RemoveFolderDetail:  "Remove a local novel folder",
				RemoveOnlineLabel:   "Remove Online Cache",
				RemoveOnlineDetail:  "Delete a cached online novel",
				CountSuffixNone:     "(none)",
				CountSuffixSingle:   "(1)",
				CountSuffixMultiple: "(%d)",
				LanguageNames: map[Locale]string{
					LocaleChinese: "Chinese",
					LocaleEnglish: "English",
				},
				LineSpacingUpdateFailed: "Failed to update line spacing: %v",
				FolderDialogUnavailable: "Folder selection dialog is not available on this system.",
				SaveConfigFailed:        "Failed to save settings: %v",
			},
			Search: SearchStrings{
				Placeholder:          "Enter a novel name…",
				Prompt:               "Search: ",
				InputHint:            "Enter a keyword and press Enter to search",
				FoundTemplate:        "Found %d books",
				LoadingTitleTemplate: "Loading \"%s\"…",
				LoadingGeneric:       "Loading novel…",
				Searching:            "Searching…",
				ErrorTemplate:        "Error: %v",
				FailedTemplate:       "Failed to load \"%s\": %v",
				SearchFailedTemplate: "Search failed: %v",
			},
			Confirm: ConfirmStrings{
				RemoveFolderPromptTemplate: "Are you sure you want to remove %s?",
				RemoveFolderConfirm:        "Confirm",
				RemoveOnlinePromptTemplate: "Remove cached novel 《%s》?",
				RemoveOnlineConfirm:        "Remove",
			},
			Bookshelf: BookshelfStrings{
				DiscoveryName: "Discover",
				UnknownType:   "Unknown item",
			},
			Reader: ReaderStrings{
				LoadingDefault:       "Loading chapter…",
				LoadingTitleTemplate: "Loading %s…",
				ChapterTemplate:      "Chapter %d",
			},
			TOC: TOCStrings{
				Title:          "Table of Contents",
				StatusSingular: "chapter",
				StatusPlural:   "chapters",
				FilterPrompt:   "Search:",
			},
			Dialog: DialogStrings{
				SelectFolderPrompt: "Select a novel folder",
			},
			Novel: NovelStrings{
				LocalPrefix: "Local",
			},
			Common: CommonStrings{
				UnknownState: "Unknown state",
			},
			Layout: LayoutStrings{
				UnderlineLength: 60,
			},
		},
	}

	availableLocales = []Locale{
		LocaleChinese,
		LocaleEnglish,
	}

	currentLocale = LocaleChinese
	current       = translations[currentLocale]
)

func AvailableLocales() []Locale {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Locale, len(availableLocales))
	copy(out, availableLocales)
	return out
}

func SetLocale(loc Locale) bool {
	mu.Lock()
	defer mu.Unlock()
	strings, ok := translations[loc]
	if !ok {
		return false
	}
	currentLocale = loc
	current = strings
	return true
}

func CurrentLocale() Locale {
	mu.RLock()
	defer mu.RUnlock()
	return currentLocale
}

func Active() *Strings {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

func LanguageName(loc Locale) string {
	s := Active()
	if name, ok := s.Settings.LanguageNames[loc]; ok {
		return name
	}
	return string(loc)
}

func ChapterTitle(index int) string {
	s := Active()
	return fmt.Sprintf(s.Reader.ChapterTemplate, index)
}

func ReaderLoadingTitle(title string) string {
	s := Active()
	return fmt.Sprintf(s.Reader.LoadingTitleTemplate, title)
}

func SearchFound(count int) string {
	s := Active()
	return fmt.Sprintf(s.Search.FoundTemplate, count)
}

func SearchLoadingTitle(title string) string {
	s := Active()
	return fmt.Sprintf(s.Search.LoadingTitleTemplate, title)
}

func SearchFailed(title string, err error) string {
	s := Active()
	return fmt.Sprintf(s.Search.FailedTemplate, title, err)
}

func SearchError(err error) string {
	s := Active()
	return fmt.Sprintf(s.Search.ErrorTemplate, err)
}

func SearchGeneralFailure(err error) string {
	s := Active()
	return fmt.Sprintf(s.Search.SearchFailedTemplate, err)
}

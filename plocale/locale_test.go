package plocale

import (
	"reflect"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func TestBundleInit(t *testing.T) {
	// Bundle is initialized in init(), verify it is not nil
	if Bundle == nil {
		t.Fatal("Bundle should not be nil after init()")
	}
}

func TestAllMessagesParseable(t *testing.T) {
	messages := collectAllMessages()
	if len(messages) == 0 {
		t.Fatal("no messages found")
	}

	for _, msg := range messages {
		result := Bundle.LocalizeAll(msg)
		if len(result) == 0 {
			t.Errorf("LocalizeAll returned empty result for message ID: %s", msg.ID)
		}
	}
}

func TestMessageIDUniqueness(t *testing.T) {
	messages := collectAllMessages()
	seen := make(map[string]bool)
	for _, msg := range messages {
		if seen[msg.ID] {
			t.Errorf("duplicate message ID: %s", msg.ID)
		}
		seen[msg.ID] = true
	}
}

func TestTomlKeysMatchMessages(t *testing.T) {
	messages := collectAllMessages()
	messageIDs := make(map[string]bool, len(messages))
	for _, msg := range messages {
		messageIDs[msg.ID] = true
	}

	cases := map[string]string{
		"active.zh.toml": "zh",
		"active.en.toml": "en",
	}

	for filename, lang := range cases {
		t.Run(lang, func(t *testing.T) {
			data, err := localeFS.ReadFile(filename)
			if err != nil {
				t.Fatalf("failed to read %s: %v", filename, err)
			}

			var tomlData map[string]interface{}
			if err := toml.Unmarshal(data, &tomlData); err != nil {
				t.Fatalf("failed to parse %s: %v", filename, err)
			}

			tomlKeys := make(map[string]bool, len(tomlData))
			for k := range tomlData {
				tomlKeys[k] = true
			}

			// Check toml has all message IDs
			for id := range messageIDs {
				if !tomlKeys[id] {
					t.Errorf("%s is missing key: %s", filename, id)
				}
			}

			// Check toml has no extra keys
			for k := range tomlKeys {
				if !messageIDs[k] {
					t.Errorf("%s has extra key not in messages: %s", filename, k)
				}
			}
		})
	}
}

// collectAllMessages uses reflection to collect all exported *i18n.Message
// variables defined in this package.
func collectAllMessages() []*i18n.Message {
	var messages []*i18n.Message

	// All messages are package-level variables. We collect them by
	// referencing them directly since Go reflection cannot enumerate
	// package-level variables. We list them all explicitly.
	all := []*i18n.Message{
		// Rule Categories
		PgRuleTypeDMLConvention,
		PgRuleTypeDDLConvention,
		PgRuleTypeDQLConvention,
		PgRuleTypeSuggestion,
		PgRuleTypeNamingConvention,
		PgRuleTypeGlobalConfig,
		PgRuleTypeIndexOptimization,
		PgRuleTypeIndexConvention,
		// pg_001
		PgRule001Desc, PgRule001Annotation, PgRule001Message,
		// pg_002
		PgRule002Desc, PgRule002Annotation, PgRule002Message,
		// pg_003
		PgRule003Desc, PgRule003Annotation, PgRule003Message,
		// pg_004
		PgRule004Desc, PgRule004Annotation, PgRule004Message,
		// pg_005
		PgRule005Desc, PgRule005Annotation, PgRule005Message,
		// pg_006
		PgRule006Desc, PgRule006Annotation, PgRule006Message,
		// pg_007
		PgRule007Desc, PgRule007Annotation, PgRule007Message,
		// pg_008
		PgRule008Desc, PgRule008Annotation, PgRule008Message,
		// pg_009
		PgRule009Desc, PgRule009Annotation, PgRule009Message, PgRule009Params1,
		// pg_010
		PgRule010Desc, PgRule010Annotation, PgRule010Message, PgRule010Params1,
		// pg_011
		PgRule011Desc, PgRule011Annotation, PgRule011Message, PgRule011Params1,
		// pg_012
		PgRule012Desc, PgRule012Annotation, PgRule012Message, PgRule012Params1,
		// pg_015
		PgRule015Desc, PgRule015Annotation, PgRule015Message,
		// pg_016
		PgRule016Desc, PgRule016Annotation, PgRule016Message,
		// pg_017
		PgRule017Desc, PgRule017Annotation, PgRule017Message, PgRule017Params1,
		// pg_019
		PgRule019Desc, PgRule019Annotation, PgRule019Message, PgRule019Params1,
		// pg_021
		PgRule021Desc, PgRule021Annotation, PgRule021Message,
		// pg_022
		PgRule022Desc, PgRule022Annotation, PgRule022Message,
		// pg_023
		PgRule023Desc, PgRule023Annotation, PgRule023Message,
		// pg_024
		PgRule024Desc, PgRule024Annotation, PgRule024Params1,
		// pg_025
		PgRule025Desc, PgRule025Annotation,
		// pg_026
		PgRule026Desc, PgRule026Annotation, PgRule026Message,
		// pg_027
		PgRule027Desc, PgRule027Annotation, PgRule027Message,
		// pg_028
		PgRule028Desc, PgRule028Annotation, PgRule028Message,
		// pg_031
		PgRule031Desc, PgRule031Annotation, PgRule031Message,
		// pg_032
		PgRule032Desc, PgRule032Annotation, PgRule032Message,
		// pg_033
		PgRule033Desc, PgRule033Annotation, PgRule033Message,
		// pg_034
		PgRule034Desc, PgRule034Annotation, PgRule034Message,
		// pg_035
		PgRule035Desc, PgRule035Annotation, PgRule035Message,
		// pg_036
		PgRule036Desc, PgRule036Annotation, PgRule036Message,
		// pg_037
		PgRule037Desc, PgRule037Annotation, PgRule037Message,
		// pg_038
		PgRule038Desc, PgRule038Annotation, PgRule038Message,
		// pg_039
		PgRule039Desc, PgRule039Annotation, PgRule039Message, PgRule039Params1,
		// pg_040
		PgRule040Desc, PgRule040Annotation, PgRule040Message,
		// pg_041
		PgRule041Desc, PgRule041Annotation, PgRule041Message, PgRule041Params1,
		// pg_042
		PgRule042Desc, PgRule042Annotation, PgRule042Message,
		// pg_043
		PgRule043Desc, PgRule043Annotation, PgRule043Message,
		// pg_044
		PgRule044Desc, PgRule044Annotation, PgRule044Message,
		// pg_045
		PgRule045Desc, PgRule045Annotation, PgRule045Message,
		// pg_047
		PgRule047Desc, PgRule047Annotation, PgRule047Message,
		// pg_048
		PgRule048Desc, PgRule048Annotation, PgRule048Message, PgRule048Params1,
		// pg_049
		PgRule049Desc, PgRule049Annotation, PgRule049Message, PgRule049Params1,
		// pg_050
		PgRule050Desc, PgRule050Annotation, PgRule050Message, PgRule050Params1,
		// pg_051
		PgRule051Desc, PgRule051Annotation, PgRule051Message,
		// pg_052
		PgRule052Desc, PgRule052Annotation, PgRule052Message, PgRule052Params1,
		// pg_053
		PgRule053Desc, PgRule053Annotation, PgRule053Message,
		// pg_054
		PgRule054Desc, PgRule054Annotation, PgRule054Message,
		// pg_055
		PgRule055Desc, PgRule055Annotation, PgRule055Message, PgRule055Params1,
		// pg_056
		PgRule056Desc, PgRule056Annotation, PgRule056Message, PgRule056Params1,
		// pg_057
		PgRule057Desc, PgRule057Annotation, PgRule057Message,
		// pg_059
		PgRule059Desc, PgRule059Annotation, PgRule059Message,
		// pg_060
		PgRule060Desc, PgRule060Annotation, PgRule060Message,
		// pg_061
		PgRule061Desc, PgRule061Annotation, PgRule061Message,
		// pg_062
		PgRule062Desc, PgRule062Annotation, PgRule062Message, PgRule062Params1,
		// pg_063
		PgRule063Desc, PgRule063Annotation, PgRule063Message,
		// pg_064
		PgRule064Desc, PgRule064Annotation, PgRule064Message, PgRule064Params1,
		// pg_065
		PgRule065Desc, PgRule065Annotation, PgRule065Message,
		// pg_066
		PgRule066Desc, PgRule066Annotation, PgRule066Message,
		// pg_067
		PgRule067Desc, PgRule067Annotation, PgRule067Message, PgRule067Params1,
		// pg_068
		PgRule068Desc, PgRule068Annotation, PgRule068Message, PgRule068Params1,
		// pg_069
		PgRule069Desc, PgRule069Annotation, PgRule069Message,
		// pg_070
		PgRule070Desc, PgRule070Annotation, PgRule070Message, PgRule070Params1,
		// pg_071
		PgRule071Desc, PgRule071Annotation, PgRule071Message,
		// pg_072
		PgRule072Desc, PgRule072Annotation, PgRule072Message, PgRule072Params1,
		// pg_073
		PgRule073Desc, PgRule073Annotation, PgRule073Message, PgRule073Params1,
		// pg_074
		PgRule074Desc, PgRule074Annotation, PgRule074Message, PgRule074Params1,
		// pg_075
		PgRule075Desc, PgRule075Annotation, PgRule075Message, PgRule075Params1,
		// pg_076
		PgRule076Desc, PgRule076Annotation, PgRule076Message,
		// pg_077
		PgRule077Desc, PgRule077Annotation, PgRule077Message, PgRule077Params1,
		// pg_078
		PgRule078Desc, PgRule078Annotation, PgRule078Message,
		// pg_079
		PgRule079Desc, PgRule079Annotation, PgRule079Message,
		// pg_080
		PgRule080Desc, PgRule080Annotation, PgRule080Message,
		PgRule080MessageLock, PgRule080MessageSchemaLock, PgRule080MessageTableLock,
	}

	for _, msg := range all {
		if msg != nil {
			messages = append(messages, msg)
		}
	}
	return messages
}

// TestCollectAllMessagesCount verifies that we have the expected number of
// messages to guard against accidentally missing entries.
func TestCollectAllMessagesCount(t *testing.T) {
	messages := collectAllMessages()
	// 8 categories + rule messages (Desc/Annotation/Message/Params)
	// We expect at least 200 messages
	if len(messages) < 200 {
		t.Errorf("expected at least 200 messages, got %d", len(messages))
	}
	t.Logf("total message count: %d", len(messages))
}

// TestMessageFieldsNotEmpty verifies that all message fields have non-empty values.
func TestMessageFieldsNotEmpty(t *testing.T) {
	messages := collectAllMessages()
	for _, msg := range messages {
		v := reflect.ValueOf(msg).Elem()
		id := v.FieldByName("ID").String()
		other := v.FieldByName("Other").String()
		if id == "" {
			t.Errorf("message has empty ID")
		}
		if other == "" {
			t.Errorf("message %s has empty Other field", id)
		}
	}
}

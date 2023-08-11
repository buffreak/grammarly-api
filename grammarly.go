package grammarly

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gorilla/websocket"
)

type GrammarlyWS struct {
	Ws       *websocket.Conn
	Response chan string
	Text     string
	Cookie   string
}

type GrammarlyParts struct {
	Text string `json:"text"`
	Meta struct {
		Label string `json:"label"`
	} `json:"meta"`
}

type GrammarlyLeftOrRight struct {
	Type         string           `json:"type"`
	Parts        []GrammarlyParts `json:"parts"`
	Alternatives []struct {
		Preview struct {
			Parts []GrammarlyParts `json:"parts"`
		} `json:"preview"`
	} `json:"alternatives"`
}

type GrammarlyResponse struct {
	MessageId     string                 `json:"messageId"`
	OutcomeScores map[string]interface{} `json:"outcomeScores"`
	Sdui          struct {
		Child struct {
			Child struct {
				Views struct {
					DefaultSuggestion struct {
						Children []struct {
							Type     string `json:"type"`
							Children []struct {
								Left  []GrammarlyLeftOrRight `json:"left"`
								Right []GrammarlyLeftOrRight `json:"right"`
							} `json:"children"`
						} `json:"children"`
					} `json:"default-suggestion"`
				} `json:"views"`
			} `json:"child"`
		} `json:"child"`
	} `json:"sdui"`
}

func (gws *GrammarlyWS) SetCookieFile(filename string) error {
	cookie, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed load cookie grammarly: %+v", err)
	}
	gws.Cookie = strings.TrimSpace(string(cookie))
	return nil
}

func (gws *GrammarlyWS) ConnectWS() error {

	ws, _, err := websocket.DefaultDialer.Dial("wss://capi.grammarly.com/freews", http.Header{
		"Origin":     {"moz-extension://f98d44e2-500b-486c-802d-28d8c4608ac5"},
		"Cookie":     {gws.Cookie},
		"User-Agent": {"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/116.0"},
		"Host":       {"capi.grammarly.com"},
	})
	if err != nil {
		return fmt.Errorf("error connecting to ws grammarly: %+v", err)
	}
	gws.Ws = ws
	gws.Response = make(chan string)
	return nil
}

func (gws *GrammarlyWS) WriteRequest(text string) error {
	var params []string = []string{
		`{"id":0,"action":"start","client":"extension_firefox","clientSubtype":"inline","clientVersion":"8.906.0","dialect":"american","docid":"f10feea2-0451-697f-fd71-27fbacd3cbbc","extDomain":"translate.google.co.id","documentContext":{},"clientSupports":["free_clarity_alerts","readability_check","filler_words_check","sentence_variety_check","vox_check","text_info","free_occasional_premium_alerts","set_goals_link","reconnect","gap_transform_card","tone_cards","user_mutes","mute_quoted_alerts","alerts_changes","ideas_gallery_link","full_sentence_rewrite_card","alerts_update","enclosing_highlight","realtime_proofit","tone_slider_card","ethical_ai_card","shorten_it","enclosing_highlight","main_start_highlight","consistency_check","super_alerts","suggested_snippets","autoapply"],"isDemoDoc":false,"sdui":{"supportedComponents":["alertsCount","alternativeChoice","alternativeSlider","applyAlerts","assistantCard","assistantFeed","behavior:strongAlertRef","block","box","button","clickableText","closeCard","column","dropDownMenuButton","focusAssistantCard","gButton","hideHighlights","highlightAlert","icon","image","inlineCard","list","nativeExperimentalGBConsistencyUpsellFooter","nativeExperimentalGBToneInsightsUpsellFooter","nativeFeedbackModal","nativeGetStartedChecklistModal","nativeInlineCardContent","nativeLearnMoreModal","nativeProofitModal","nativeSettingsModal","nativeToneInsightsModal","nextCard","notify","openCreateSnippetModal","openFeedback","openLearnMore","openLink","openSettings","openToneDetector","popAssistantFeed","prevCard","proofitButton","pushAssistantFeed","removeAlerts","removeRoot","row","scroll","selectAlternative","showHighlights","slider","strongAlertRef","switchView","text","transition","upgradeToPremium","viewStack"],"protocol":"2","dslSchema":"4.18.1"},"containerType":"form field"}`,
		`{"id":1,"action":"submit_ot","rev":0,"doc_len":0,"chunked":false,"timer":{"client_clock":249621,"id":"cd4eebc7-3408-4083-a2d9-5c0f23b8399f"},"deltas": [{"ops":[{"insert":"` + text + `"}]}]}`,
	}

	gws.Text = text
	for key, param := range params {
		err := gws.Ws.WriteMessage(websocket.TextMessage, []byte(param))
		if err != nil {
			return fmt.Errorf("error send message to ws grammarly at index [%d]: %+v", key, err)
		}
	}
	return nil
}

func (gws *GrammarlyWS) readResponse() error {
	for {
		_, msg, err := gws.Ws.ReadMessage()
		if err != nil {
			return fmt.Errorf("error read message to ws grammarly: %+v", err)
		}
		gws.Response <- string(msg)
	}
}

func (gws *GrammarlyWS) ParseResponse() (string, error) {
	defer gws.Ws.Close()
	go gws.readResponse()
	for {
		var grammarlyResp = GrammarlyResponse{}
		buffer := <-gws.Response
		if err := json.Unmarshal([]byte(buffer), &grammarlyResp); err != nil {
			fmt.Printf("error parse response ws from grammarly: %+v\n", err)
			continue
		}

		if len(grammarlyResp.OutcomeScores) > 0 {
			break
		}

		for _, data := range grammarlyResp.Sdui.Child.Child.Views.DefaultSuggestion.Children {
			if data.Type == "column" {
				for _, child := range data.Children {
					var subsets = [][]GrammarlyLeftOrRight{child.Left, child.Right} // left has greater priority
					for _, subset := range subsets {
						for _, sub := range subset {
							var listElement = []GrammarlyParts{}
							if sub.Type == "block" {
								listElement = sub.Parts
							} else if sub.Type == "alternativeChoice" {
								if len(sub.Alternatives) > 0 {
									listElement = sub.Alternatives[0].Preview.Parts
								}
							}
							regexRemoveChar := regexp.MustCompile(`…`)
							var startText string = regexRemoveChar.ReplaceAllString(listElement[0].Text, "")
							var endText string = regexRemoveChar.ReplaceAllString(listElement[len(listElement)-1].Text, "")
							var regexInlineWord *regexp.Regexp = regexp.MustCompile(startText + `(.*?)` + endText)
							var replacement string
							for i := 1; i < len(listElement)-1; i++ {
								if regexp.MustCompile("(?mi)^insert line break").MatchString(listElement[i].Meta.Label) {
									// this statement will be fix soon with counter minimum words per paragraph
									replacement = "\n"
									// or
									// replacement = " " // just add space and continue the lines
								} else if regexp.MustCompile("(?mi)^(insert) ").MatchString(listElement[i].Meta.Label) {
									replacement = listElement[i].Text
								} else if regexp.MustCompile("(?mi)^(delete|remove) ").MatchString(listElement[i].Meta.Label) {
									replacement = ""
								}
							}
							regexRemoveMultiSpace := regexp.MustCompile(`( {2,})`)
							gws.Text = regexRemoveMultiSpace.ReplaceAllString(regexInlineWord.ReplaceAllString(gws.Text, startText+replacement+endText), " ")
						}
					}
				}
			}
		}
	}
	return gws.Text, nil
}

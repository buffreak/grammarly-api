package grammarly

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var (
	ReadResponseErr error
)

type Configuration struct {
	WithNewline     bool
	NewlineOverride string
}

type GrammarlyWS struct {
	Ws       *websocket.Conn
	Response chan string
	Text     string
	Cookie   string
	Configuration
}

type GrammarlyParts struct {
	Text string `json:"text"`
	Meta struct {
		Label string `json:"label"`
	} `json:"meta"`
	TextColor string   `json:"textColor"`
	Format    []string `json:"format"`
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
	ScoresStatus  string                 `json:"scoresStatus"`
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

type Correction struct {
	Text         string
	DeletedText  map[string]string
	InsertedText map[string]string
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
	gws.Ws.SetReadLimit(int64(math.Pow(2, 32)))
	gws.Response = make(chan string)
	return nil
}

func (gws *GrammarlyWS) WriteRequest(text string) error {

	random := rand.New(rand.NewSource(time.Now().UnixNano()))

	p1 := random.Intn(100) + 1
	p2 := random.Intn(200-101+1) + 101

	var params []string = []string{
		`{"id":` + strconv.Itoa(p1) + `,"action":"start","client":"extension_firefox","clientSubtype":"inline","clientVersion":"8.906.0","dialect":"american","docid":"f10feea2-0451-697f-fd71-` + strconv.Itoa(p1) + `fbacd3cbbc","extDomain":"translate.google.co.id","documentContext":{},"clientSupports":["free_clarity_alerts","readability_check","filler_words_check","sentence_variety_check","vox_check","text_info","free_occasional_premium_alerts","set_goals_link","reconnect","gap_transform_card","tone_cards","user_mutes","mute_quoted_alerts","alerts_changes","ideas_gallery_link","full_sentence_rewrite_card","alerts_update","enclosing_highlight","realtime_proofit","tone_slider_card","ethical_ai_card","shorten_it","enclosing_highlight","main_start_highlight","consistency_check","super_alerts","suggested_snippets","autoapply"],"isDemoDoc":false,"sdui":{"supportedComponents":["alertsCount","alternativeChoice","alternativeSlider","applyAlerts","assistantCard","assistantFeed","behavior:strongAlertRef","block","box","button","clickableText","closeCard","column","dropDownMenuButton","focusAssistantCard","gButton","hideHighlights","highlightAlert","icon","image","inlineCard","list","nativeExperimentalGBConsistencyUpsellFooter","nativeExperimentalGBToneInsightsUpsellFooter","nativeFeedbackModal","nativeGetStartedChecklistModal","nativeInlineCardContent","nativeLearnMoreModal","nativeProofitModal","nativeSettingsModal","nativeToneInsightsModal","nextCard","notify","openCreateSnippetModal","openFeedback","openLearnMore","openLink","openSettings","openToneDetector","popAssistantFeed","prevCard","proofitButton","pushAssistantFeed","removeAlerts","removeRoot","row","scroll","selectAlternative","showHighlights","slider","strongAlertRef","switchView","text","transition","upgradeToPremium","viewStack"],"protocol":"2","dslSchema":"4.18.1"},"containerType":"form field"}`,
		`{"id":` + strconv.Itoa(p2) + `,"action":"submit_ot","rev":` + strconv.Itoa(p1) + `,"doc_len":0,"chunked":false,"timer":{"client_clock":249621,"id":"cd4eebc7-3408-4083-a2d9-5c0f23b83` + strconv.Itoa(p2) + `f"},"deltas": [{"ops":[{"insert":"` + text + `"}]}]}`,
	}

	gws.Text = text
	for key, param := range params {
		var body interface{}
		err := json.Unmarshal([]byte(param), &body)
		if err != nil {
			return fmt.Errorf("error parse request message to ws grammarly at index [%d]: %+v", key, err)
		}
		err = gws.Ws.WriteJSON(body)
		if err != nil {
			return fmt.Errorf("error send message to ws grammarly at index [%d]: %+v", key, err)
		}
	}

	return nil
}

func (gws *GrammarlyWS) ParseResponse() (string, error) {
	regexRemoveMultiSpace := regexp.MustCompile(`( {2,})`)
	for {
		_, msg, err := gws.Ws.ReadMessage()
		if err != nil {
			fmt.Printf("error read response grammarly: %s\n", err.Error())
			break
		}
		var grammarlyResp = GrammarlyResponse{}
		buffer := string(msg)
		fmt.Printf("%s\n", buffer)
		if err := json.Unmarshal([]byte(buffer), &grammarlyResp); err != nil {
			fmt.Printf("error parse response ws from grammarly: %+v\n", err)
			continue
		}

		if len(grammarlyResp.OutcomeScores) > 0 || grammarlyResp.ScoresStatus == "TOO_SMALL" {
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
							var correction = Correction{}
							correction.InsertedText = make(map[string]string)
							correction.DeletedText = make(map[string]string)
							var sequence string
							for i := 0; i < len(listElement); i++ {
								if listElement[i].Meta.Label == "" && listElement[i].Text != " " && listElement[i].TextColor == "CoreNeutral90" {
									correction.Text += regexRemoveChar.ReplaceAllString(listElement[i].Text, "")

									if i == len(listElement)-1 || i == len(listElement)-2 {
										sequence += regexRemoveChar.ReplaceAllString(listElement[i].Text, "")
									} else {
										sequence += regexRemoveChar.ReplaceAllString(listElement[i].Text, "") + "(.*?)"
									}
								}

								if regexp.MustCompile("(?mi)^insert line break").MatchString(listElement[i].Meta.Label) {
									continue
								} else if regexp.MustCompile("(?mi)^(insert) ").MatchString(listElement[i].Meta.Label) {
									correction.InsertedText[listElement[i].Meta.Label] = listElement[i].Text
									correction.Text += listElement[i].Text
								} else if regexp.MustCompile("(?mi)^(delete|remove) ").MatchString(listElement[i].Meta.Label) {
									correction.DeletedText[listElement[i].Meta.Label] = listElement[i].Text
									correction.Text = strings.ReplaceAll(correction.Text, listElement[i].Text, " ")
								}
							}
							gws.Text = regexRemoveMultiSpace.ReplaceAllString(regexp.MustCompile(sequence).ReplaceAllString(gws.Text, correction.Text), " ")
							gws.Text = regexp.MustCompile(`([a-zA-Z0-9])[.,]([a-zA-Z0-9])`).ReplaceAllString(gws.Text, `$1 $2`)
						}
					}
				}
			}
		}
	}
	return gws.Text, nil
}

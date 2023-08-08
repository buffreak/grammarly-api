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
	Ws             *websocket.Conn
	Response       chan string
	Text           string
	CookieFileName string
}

func (gws *GrammarlyWS) SetCookiePath(filename string) error {
	cookie, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed load cookie grammarly: %+v", err)
	}
	gws.CookieFileName = strings.TrimSpace(string(cookie))
	return nil
}

func (gws *GrammarlyWS) ConnectWS() error {

	ws, _, err := websocket.DefaultDialer.Dial("wss://capi.grammarly.com/freews", http.Header{
		"Origin":     {"moz-extension://f98d44e2-500b-486c-802d-28d8c4608ac5"},
		"Cookie":     {gws.CookieFileName},
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
		var body map[string]interface{}
		buffer := <-gws.Response
		if err := json.Unmarshal([]byte(buffer), &body); err != nil {
			fmt.Printf("error parse response ws from grammarly: %+v\n", err)
			continue
		}
		if _, ok := body["messageId"]; ok {
			if sdui, ok := body["sdui"].(map[string]interface{}); ok {
				if child, ok := sdui["child"].(map[string]interface{}); ok {
					if child, ok := child["child"].(map[string]interface{}); ok {
						if views, ok := child["views"].(map[string]interface{}); ok {
							if defaultSuggestion, ok := views["default-suggestion"].(map[string]interface{}); ok {
								if children, ok := defaultSuggestion["children"].([]interface{}); ok {
									for _, data := range children {
										if row, ok := data.(map[string]interface{}); ok {
											if kindOf, ok := row["type"].(string); ok {
												if kindOf == "column" {
													if childrens, ok := row["children"].([]interface{}); ok {
														for _, child := range childrens {
															if data, ok := child.(map[string]interface{}); ok {
																var subsets = []string{"left", "right"} // left has greater priority
																for _, subset := range subsets {
																	if dataSub, ok := data[subset].([]interface{}); ok {
																		for _, sub := range dataSub {
																			var listElement interface{}
																			if data, ok := sub.(map[string]interface{}); ok {
																				if parseType, ok := data["type"].(string); ok {
																					if parseType == "block" {
																						listElement = data["parts"]
																					} else if parseType == "alternativeChoice" {
																						if alternatives, ok := data["alternatives"].([]interface{}); ok {
																							if preview, ok := alternatives[0].(map[string]interface{})["preview"].(map[string]interface{}); ok {
																								listElement = preview["parts"]
																							}
																						}
																					}
																				}
																				if parts, ok := listElement.([]interface{}); ok {
																					var startText string = strings.Replace(parts[0].(map[string]interface{})["text"].(string), "…", "", -1)
																					var endText string = strings.Replace(parts[len(parts)-1].(map[string]interface{})["text"].(string), "…", "", -1)
																					var regexInlineWord *regexp.Regexp = regexp.MustCompile(startText + `(.*?)` + endText)
																					var replacement string
																					for i := 1; i < len(parts)-1; i++ {
																						if section, ok := parts[i].(map[string]interface{}); ok {
																							if meta, ok := section["meta"].(map[string]interface{}); ok {
																								if label, ok := meta["label"].(string); ok {
																									if regexp.MustCompile("(?mi)^insert line break").MatchString(label) {
																										replacement = "\n"
																										// or
																										// replacement = " " // just add space and continue the lines
																									} else if regexp.MustCompile("(?mi)^(insert) ").MatchString(label) {
																										replacement = section["text"].(string)
																									} else if regexp.MustCompile("(?mi)^(delete|remove) ").MatchString(label) {
																										replacement = ""
																									}
																								}
																							}
																						}
																					}
																					gws.Text = regexInlineWord.ReplaceAllString(gws.Text, startText+replacement+endText)
																				}
																			}
																		}
																	}
																}
															}
														}
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		} else if _, ok := body["outcomeScores"]; ok {
			break
		}
	}
	return gws.Text, nil
}

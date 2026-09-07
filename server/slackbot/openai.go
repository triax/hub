package slackbot

import (
	"context"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// 使うモデルはここに集約する。focus は要約の質が要るので gpt-4o、
// 翻訳と echo は短文なので mini で足りる。
const (
	chatModelFocus = "gpt-4o"
	chatModelLight = "gpt-4o-mini"
)

// openAIChat は ChatGPT インタフェースの OpenAI 公式 SDK 実装。
// SDK の型はこのファイルの外に出さない。
type openAIChat struct {
	client openai.Client
}

// NewOpenAIChat は OPENAI_API_KEY から ChatGPT を作る。
// キーが空のときは nil を返す（呼び出し側は ChatGPT == nil を
// 「LLM が使えない環境」として扱う。ローカル開発の既定）。
func NewOpenAIChat(apiKey string) ChatGPT {
	if apiKey == "" {
		return nil
	}
	return newOpenAIChat(option.WithAPIKey(apiKey))
}

// newOpenAIChat は option を直接受ける非公開のコンストラクタ。
// テストが option.WithBaseURL で送信先を差し替えるために分けてある。
func newOpenAIChat(opts ...option.RequestOption) *openAIChat {
	return &openAIChat{client: openai.NewClient(opts...)}
}

func (c *openAIChat) Chat(ctx context.Context, req ChatRequest) (string, error) {
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(req.System)+1)
	for _, s := range req.System {
		messages = append(messages, openai.SystemMessage(s))
	}
	messages = append(messages, openai.UserMessage(req.User))

	params := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(req.Model),
		Messages: messages,
	}
	if req.Schema != nil {
		params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   req.Schema.Name,
					Schema: req.Schema.Schema,
					Strict: openai.Bool(true),
				},
			},
		}
	}

	res, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", err
	}
	if len(res.Choices) == 0 {
		return "", fmt.Errorf("要約が返ってきませんでした")
	}
	if refusal := res.Choices[0].Message.Refusal; refusal != "" {
		return "", fmt.Errorf("モデルが応答を拒否しました: %s", refusal)
	}
	return res.Choices[0].Message.Content, nil
}

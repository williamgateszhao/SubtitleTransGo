package main

import (
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// Config struct for storing CLI arguments and flags.
type Config struct {
	sourceSrt            string
	destSrt              string
	referenceSrt         string
	translator           string
	apiUrl               string
	apiKey               string
	modelName            string
	botId                string
	systemPrompt         string
	systemPrompt2        string
	userPrompt           string
	userPrompt2          string
	userPrompt3          string
	temperature          float32
	topP                 float32
	frequencyPenalty     float32
	maxTokens            int
	maxRequestsPerMinute int
	use2ndSystemPrompt   bool
	use2ndUserPrompt     bool
	bilingual            bool
	preProcessing1       bool
	preProcessing2       bool
	preProcessing3       bool
	postProcessing1      bool
}

// Config struct for storing CLI arguments and flags.
type SrtSegment struct {
	ID   string
	Time string
	Text string
	Err  error
}

func main() {
	var config Config
	var result []SrtSegment

	var rootCmd = &cobra.Command{
		Use:   "stgo <COMMAND>",
		Short: "Subtitle translation and processing tool written in Go",
		Long:  `Subtitle translation and processing tool written in Go`,
		Args:  cobra.ExactArgs(1),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			config.sourceSrt = args[0]
			// Automatically sets the destination file based on the source file if not explicitly provided.
			if config.destSrt == "" {
				config.destSrt = strings.TrimSuffix(config.sourceSrt, filepath.Ext(config.sourceSrt)) + "." + config.translator + ".translated" + filepath.Ext(config.sourceSrt)
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			srtFile, err := readSrtFile(config.sourceSrt)
			checkError(err)

			if config.preProcessing1 {
				srtFile = reduceRepeatedPatterns(srtFile)
			}
			if config.preProcessing2 {
				srtFile = deleteSrtSegmentsOnlyContainsRepeatedCharacters(srtFile)
			}
			if config.preProcessing3 {
				srtFile = extendSegments(srtFile)
			}
			if config.use2ndSystemPrompt {
				config.systemPrompt = config.systemPrompt2
			}
			if config.use2ndUserPrompt {
				config.userPrompt = config.userPrompt2
			}
			if config.referenceSrt != "" {
				config.userPrompt = config.userPrompt3
				result, err = readSrtFile(config.referenceSrt)
				checkError(err)
			}

			switch config.translator {
			case "google":
				result = translateSrtSegmentsInBatches(srtFile, &config)
			case "openai":
				if result != nil {
					result = translateSrtSegmentsByLine(srtFile, &config, result)
				} else {
					result = translateSrtSegmentsByLine(srtFile, &config)
				}
			case "coze":
				if result != nil {
					result = translateSrtSegmentsByLine(srtFile, &config, result)
				} else {
					result = translateSrtSegmentsByLine(srtFile, &config)
				}
			}

			if config.postProcessing1 {
				result = trimAnnotation(srtFile, result)
			}

			err = saveSrtFile(result, srtFile, config.destSrt, config.bilingual)
			checkError(err)

		},
	}
	rootCmd.PersistentFlags().StringVar(&config.destSrt, "dest", "", "Path to the destination SRT file for writing.")
	rootCmd.PersistentFlags().StringVar(&config.referenceSrt, "reference", "", "Path to the SRT file for reference.")
	rootCmd.PersistentFlags().StringVar(&config.translator, "translator", "google", "Specifies the translation service to use, options: 'openai', 'coze', or 'google'. The 'openai' value indicates compatibility with OpenAI-based APIs.")
	rootCmd.PersistentFlags().StringVar(&config.apiUrl, "apiurl", "", "The URL endpoint for the translation API. Not required for the 'google' translator option.")
	rootCmd.PersistentFlags().StringVar(&config.apiKey, "apikey", "", "The access key for the translation API. Not required for the 'google' translator option.")
	rootCmd.PersistentFlags().StringVar(&config.modelName, "model", "", "Translation model to be used, required only for 'openai' translator.")
	rootCmd.PersistentFlags().StringVar(&config.botId, "bot", "", "Identifies the bot to be used, applicable only for 'coze' translator.")
	rootCmd.PersistentFlags().StringVar(&config.systemPrompt, "systemprompt", "你是影视剧台词翻译专家，你将用户提供的台词准确地翻译成目标语言，既保持忠于原文，又要尽量符合目标语言的语法和表达习惯，口吻偏口语化，译文不要包含任何非目标语言的字词。如果原文没有提供人称代词，请不要擅自推测和添加人称代词。你的回答应该只包含译文，绝对不包含其他任何内容，也不包含“以下是我的翻译”等语句，特别注意不要对台词内容进行评价。", "System prompt provided to the AI.")
	rootCmd.PersistentFlags().StringVar(&config.systemPrompt2, "systemprompt2", "You are an expert in translating movie and TV show scripts. You will translate the lines provided by the user accurately into the target language, remaining faithful to the original text while conforming as much as possible to the target language's grammar and idiomatic expressions. The tone should be colloquial, and the translation should not include any words from languages other than the target language. If the original text does not provide personal pronouns, do not presume to guess and add them. Your response should only include the translation, absolutely no other content, and specifically avoid any commentary on the dialogue content.", "System prompt provided to the AI (no effect unless use2ndsystemprompt is true).")
	rootCmd.PersistentFlags().StringVar(&config.userPrompt, "userprompt", "请将以下台词翻译为中文台词：<ot>", "User prompt provided to the AI, Use '<ot>' as the placeholder in the template to represent the original text to be translated, and '<rt>' to represent the reference translation if any.")
	rootCmd.PersistentFlags().StringVar(&config.userPrompt2, "userprompt2", "Please translate the following lines into Chinese: <ot>", "User prompt provided to the AI, Use '<ot>' as the placeholder in the template to represent the original text to be translated, and '<rt>' to represent the reference translation if any. (no effect unless use2nduserprompt is true)")
	rootCmd.PersistentFlags().StringVar(&config.userPrompt3, "userprompt3", "需要翻译的是以下台词：<ot>\n其他人将其翻译为：<rt>\n请积极参考其他人的翻译，将上述台词翻译为中文台词：", "User prompt provided to the AI, Use '<ot>' as the placeholder in the template to represent the original text to be translated, and '<rt>' to represent the reference translation if any. (no effect unless reference is set)")
	rootCmd.PersistentFlags().Float32Var(&config.temperature, "temperature", 0.7, "Temperature setting for the AI.")
	rootCmd.PersistentFlags().Float32Var(&config.topP, "topp", 0.95, "Top_P setting for the AI.")
	rootCmd.PersistentFlags().Float32Var(&config.frequencyPenalty, "frequencypenalty", 0.0, "Frequency_penalty setting for the AI.")
	rootCmd.PersistentFlags().IntVar(&config.maxTokens, "maxtokens", 1024, "The maximum number of tokens contained in each line of translated subtitles.")
	rootCmd.PersistentFlags().IntVar(&config.maxRequestsPerMinute, "maxrpm", 20, "The maximum number of translation requests permitted per minute.")
	rootCmd.PersistentFlags().BoolVar(&config.use2ndSystemPrompt, "use2ndsystemprompt", false, "Use 2nd system prompt.")
	rootCmd.PersistentFlags().BoolVar(&config.use2ndUserPrompt, "use2nduserprompt", false, "Use 2nd user prompt.")
	rootCmd.PersistentFlags().BoolVar(&config.bilingual, "bilingual", false, "Enables saving both the original and translated subtitles in the destination SRT file.")
	rootCmd.PersistentFlags().BoolVar(&config.preProcessing1, "pre1", false, "Preprocessing method 1: Reduces repeated patterns of 2 to 4 characters in subtitles down to two instances.")
	rootCmd.PersistentFlags().BoolVar(&config.preProcessing2, "pre2", false, "Preprocessing method 2: Removes subtitles that consist only of repeated Unicode characters.")
	rootCmd.PersistentFlags().BoolVar(&config.preProcessing3, "pre3", false, "Preprocessing method 3: If the duration of a subtitle line is less than 1.2 seconds, extend it to 1.2 seconds, without exceeding the start time of the next subtitle line.")
	rootCmd.PersistentFlags().BoolVar(&config.postProcessing1, "post1", false, "Postprocessing method 1: Discard line breaks and subsequent content if the translation has more line breaks than the original text.")

	err := rootCmd.Execute()
	checkError(err)
}

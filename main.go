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
	userPrompt           string
	temperature          float32
	topP                 float32
	maxTokens            int
	maxRequestsPerMinute int
	bilingual            bool
	preProcessing1       bool
	preProcessing2       bool
	postProcessing1      bool
}

// Config struct for storing CLI arguments and flags.
type SrtSegment struct {
	ID   string
	Time string
	Text string
}

func main() {
	var config Config
	var result []SrtSegment

	var rootCmd = &cobra.Command{
		Use:   "stgo <COMMAND>",
		Short: "Subtitle Translatlor written in go",
		Long:  `Subtitle Translatlor written in go`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if config.sourceSrt == "" && args[0] != "" {
				config.sourceSrt = args[0]
			}
			if config.destSrt == "" {
				config.destSrt = strings.TrimSuffix(config.sourceSrt, filepath.Ext(config.sourceSrt)) + "." + config.translator + ".translated" + filepath.Ext(config.sourceSrt)
			} // Auto set destination file based on the source file if not provided.

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

			if config.referenceSrt != "" {
				config.userPrompt = "需要翻译的是以下台词： <ot>\n其他人将其翻译为： <rt>\n请积极参考其他人的翻译，将上述台词翻译为中文台词： "
				result, err = readSrtFile(config.referenceSrt)
				checkError(err)
			}

			switch config.translator {
			case "google":
				result, err = translateSrtSegmentsInBatches(srtFile, &config)
				checkError(err)
			case "openai":
				if result != nil {
					result, err = translateSrtSegmentsByLine(srtFile, &config, result)
				} else {
					result, err = translateSrtSegmentsByLine(srtFile, &config)
				}
				checkError(err)
			case "coze":
				if result != nil {
					result, err = translateSrtSegmentsByLine(srtFile, &config, result)
				} else {
					result, err = translateSrtSegmentsByLine(srtFile, &config)
				}
				checkError(err)
			}

			if config.postProcessing1 {
				result = trimAnnotation(srtFile, result)
			}

			err = saveSrtFile(result, srtFile, config.destSrt, config.bilingual)
			checkError(err)

		},
	}
	rootCmd.PersistentFlags().StringVar(&config.sourceSrt, "source", "", "Path to the source SRT file for reading.")
	rootCmd.PersistentFlags().StringVar(&config.destSrt, "dest", "", "Path to the destination SRT file for writing.")
	rootCmd.PersistentFlags().StringVar(&config.referenceSrt, "reference", "", "Path to the SRT file for reference.")
	rootCmd.PersistentFlags().StringVar(&config.translator, "translator", "google", "Specifies the translation service to use, options: 'openai', 'coze', or 'google'. The 'openai' value indicates compatibility with OpenAI-based APIs.")
	rootCmd.PersistentFlags().StringVar(&config.apiUrl, "apiurl", "", "The URL endpoint for the translation API. Not required for the 'google' translator option.")
	rootCmd.PersistentFlags().StringVar(&config.apiKey, "apikey", "", "The access key for the translation API. Not required for the 'google' translator option.")
	rootCmd.PersistentFlags().StringVar(&config.modelName, "model", "", "Translation model to be used, required only for 'openai' translator.")
	rootCmd.PersistentFlags().StringVar(&config.botId, "bot", "", "Identifies the bot to be used, applicable only for 'coze' translator.")
	rootCmd.PersistentFlags().StringVar(&config.systemPrompt, "systemprompt", "你是台词翻译专家，你将用户提供的台词准确地翻译成目标语言，既保持忠于原文，又要尽量符合目标语言的语法和表达习惯，口吻偏口语化，译文不要包含任何非目标语言的字词。如果原文没有提供人称代词，请不要擅自推测和添加人称代词。你的回答应该只包含译文，绝对不包含其他任何内容，也不包含“以下是我的翻译”等语句，特别注意不要对台词内容进行评价。", "System prompt provided to the AI.")
	//a better prompt for chatgpt: "# 角色\n你是一位语言翻译专家，擅长根据用户要求进行不同类型和风格的语言翻译。\n\n## 技能\n### 技能1：按要求翻译\n- 当用户明确指定你将某种语言翻译成另一种语言时，详细捉摸用户的指示并按照用户的要求进行翻译。\n\n### 技能2：默认翻译\n- 如果用户没有提出特殊要求，你将把所有非中文的输入默认翻译成简体中文。\n\n### 技能3：风格切换\n- 用户可以指定翻译的风格，你需准确理解并在翻译过程中做到风格切换，例如台词风格，学术论文风格，新闻报道风格等。如果用户没有指定，则使用台词风格。\n\n## 约束条件\n- 你只能进行语言翻译相关的任务，如果用户有其他类别的需求，不应进行回答。\n- 你只能输出译文内容，不能对译文进行分析、讲解和评价，也不能添加类似于“以下是我翻译的内容”这样的内容。\n- 你应尽量做到准确翻译，遵循译文的原始含义，既保持忠于原文，又要根据目标语言的习惯和规则，做到通顺、自然、雅致。\n- 在翻译日语时，如果原文不包括代词，你应注意不要猜测和添加原文不存在的代词\n- 开始翻译之前，你应确认好翻译的目标语言及风格，避免出现理解错误。\n- 尽量在短时间内完成翻译，以满足用户的需求。"
	rootCmd.PersistentFlags().StringVar(&config.userPrompt, "userprompt", "请将以下台词翻译为中文台词： \n<ot>", "User prompt provided to the AI, Use '<ot>' as the placeholder in the template to represent the original text to be translated, and '<rt>' to represent the reference translation if any.")
	rootCmd.PersistentFlags().Float32Var(&config.temperature, "temperature", 0.3, "Temperature setting for the AI.")
	rootCmd.PersistentFlags().Float32Var(&config.topP, "topp", 0.95, "Top_P setting for the AI.")
	rootCmd.PersistentFlags().IntVar(&config.maxTokens, "maxtokens", 1024, "The maximum number of tokens contained in each line of translated subtitles.")
	rootCmd.PersistentFlags().IntVar(&config.maxRequestsPerMinute, "maxrpm", 10, "The maximum number of translation requests permitted per minute.")
	rootCmd.PersistentFlags().BoolVar(&config.bilingual, "bilingual", false, "Enables saving both the original and translated subtitles in the destination SRT file.")
	rootCmd.PersistentFlags().BoolVar(&config.preProcessing1, "pre1", false, "Preprocessing method 1: Reduces any 2 to 4 character long patterns (words) that appear more than twice in subtitles to two occurrences.")
	rootCmd.PersistentFlags().BoolVar(&config.preProcessing2, "pre2", false, "Preprocessing method 2: Removes subtitles that consist only of repeated Unicode characters.")
	rootCmd.PersistentFlags().BoolVar(&config.postProcessing1, "post1", false, "Preprocessing method 1: If the translation has more line breaks than the original text, discard the line break and the content that follows it.")

	err := rootCmd.Execute()
	checkError(err)
}

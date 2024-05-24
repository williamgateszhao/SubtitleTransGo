# SubtitleTransGO

一个`srt`字幕文件机翻和处理工具，只提供命令行，不适合初级用户。

An SRT subtitle file machine translation and processing tool, offering only a command-line interface, not suitable for novice users.

- 当使用AI翻译时，将字幕一行行分开发给AI，避免超越上下文限制，避免AI拒绝翻译，速度较慢
- 当使用google翻译时，自动以每段不超过5000个字符分切字幕，速度较快
- 支持google翻译（免API）
- 支持OpenAI API（ChatGPT或兼容的API）
- 支持Coze API
- 可选预处理1: 当一个长度为2-4字符之间的词在一行字幕中连续重复出现三次以上，则将其减少为连续重复两次
- 可选预处理2：当一行字幕中只包含一个字符的重复，则将这行字幕删除
- 可选预处理3：当一行字幕持续时间小于1.2秒，则延长到1.2秒，但不会超过下一条字幕的起始时间
- 可选后处理1：当译文的换行数多于原文的换行数，抛弃多出的换行及此后的内容（用于抛弃某些模型——例如Mixtral——自作主张的注释）
- 可在使用AI进行翻译时提供参考译本，提高翻译质量（建议由google翻译生成参考译本）
- When using AI translation, subtitles are sent to the AI line by line to avoid exceeding context limits and to prevent AI from refusing to translate, though this method is slower
- When using Google Translate, subtitles are automatically divided into segments no longer than 5000 characters each, resulting in faster translation speeds
- Supports Google Translate (API-free)
- Supports OpenAI API (ChatGPT or compatible APIs)
- Supports Coze API
- Optional preprocessing 1: Reduces repeated patterns of 2 to 4 characters in subtitles down to two instances
- Optional preprocessing 2: Removes subtitles that consist only of repeated Unicode characters
- Optional preprocessing 3: If the duration of a subtitle line is less than 1.2 seconds, extend it to 1.2 seconds, without exceeding the start time of the next subtitle line
- Optional post-processing 1: Discard line breaks and subsequent content if the translation has more line breaks than the original text
- Reference translations can be provided during AI translation to improve translation quality (Google Translate-generated reference translations recommended)

## 使用之前，从语音转写产生字幕文件 Before using, generate subtitle files from voice transcription

你应该先将待翻译的媒体文件转换为16Khz的wav，有助于提高转写准确性，例如

You should first convert the media file to be translated into a 16kHz WAV format to improve transcription accuracy, for example

```
ffmpeg -i input.mp4 -vn -acodec pcm_s16le -ar 16000 -ac 2 ./output.wav
```

然后你应该使用自己喜欢的转写工具和模型，生成srt文件，例如（注意下面的`vad_threshold`/`vad_onset`/`vad_offset`和`vad_max_speech_duration_s`/`chunk_size`取不同的值，对听写结果的句子长度影响很大，可根据样本自行调试）

Then you should use your preferred transcription tool and model to generate an SRT file, for example (note that taking different values for `vad_threshold`/`vad_onset`/`vad_offset` and `vad_max_speech_duration_s`/`chunk_size` greatly affects the sentence length of the transcription results, which can be adjusted based on the sample).

```
whisper-ctranslate2 --device cuda --model_directory ./whisper-large-v2-japanese --language ja --output_format srt --word_timestamps true --hallucination_silence_threshold 3 --beam_size 5 --vad_filter true --vad_threshold 0.5 --vad_min_silence_duration_ms 500 --vad_max_speech_duration_s 9 --no_speech_threshold 0.3 output.wav
```

```
whisperx --model zh-plus/faster-whisper-large-v2-japanese-5k-steps --align_model TKU410410103/wav2vec2-base-japanese-asr --output_format srt --language ja --vad_onset 0.5 --vad_offset 0.363 --chunk_size 9 --no_speech_threshold 0.3 --print_progress True output.wav
```

## 如何使用 How to use

例如

For example

```
stgo ./input.srt --reference=./google.srt --pre1 --pre2 --post1 --translator=openai --apiurl=http://localhost:11434/v1/chat/completions --apikey=xxx --model=qwen:32b-chat-v1.5-q2_K --maxrpm=50 --temperature=0.7 --topp=0.95
```

支持环境变量中的`http_proxy`和`https_proxy`设置。

Supports the settings of http_proxy and https_proxy in the environment variables.

在命令行中使用`-h`查看可选参数。

Use -h in the command line to view optional parameters.


```
Usage:
  stgo <COMMAND> [flags]

Flags:
      --apikey string              The access key for the translation API. Not required for the 'google' translator option.
      --apiurl string              The URL endpoint for the translation API. Not required for the 'google' translator option.
      --bilingual                  Enables saving both the original and translated subtitles in the destination SRT file.
      --bot string                 Identifies the bot to be used, applicable only for 'coze' translator.
      --dest string                Path to the destination SRT file for writing.
      --frequencypenalty float32   Frequency_penalty setting for the AI.
  -h, --help                       help for stgo
      --maxrpm int                 The maximum number of translation requests permitted per minute. (default 20)
      --maxtokens int              The maximum number of tokens contained in each line of translated subtitles. (default 1024)
      --model string               Translation model to be used, required only for 'openai' translator.
      --post1                      Postprocessing method 1: Discard line breaks and subsequent content if the translation has more line breaks than the original text.
      --pre1                       Preprocessing method 1: Reduces repeated patterns of 2 to 4 characters in subtitles down to two instances.
      --pre2                       Preprocessing method 2: Removes subtitles that consist only of repeated Unicode characters.
      --pre3                       Preprocessing method 3: If the duration of a subtitle line is less than 1.2 seconds, extend it to 1.2 seconds, without exceeding the start time of the next subtitle line.
      --reference string           Path to the SRT file for reference.
      --systemprompt string        System prompt provided to the AI. (default "你是影视剧台词翻译专家，你将用户提供的台词准确地翻译成目标语言，既保持忠于原文，又要尽量符合目标语言的语法和表达习惯，口吻偏口语化，译文不要包含任何非目标语言的字词。如果原文没有提供人称代词，请不要擅自推测和添加人称代词。你的回答应该只包含译文，绝对不包含其他任何内容，也不包含“以下是我的翻译”等语句，特别注意不要对台词内容进行评价。")
      --systemprompt2 string       System prompt provided to the AI (no effect unless use2ndsystemprompt is true). (default "You are an expert in translating movie and TV show scripts. You will translate the lines provided by the user accurately into the target language, remaining faithful to the original text while conforming as much as possible to the target language's grammar and idiomatic expressions. The tone should be colloquial, and the translation should not include any words from languages other than the target language. If the original text does not provide personal pronouns, do not presume to guess and add them. Your response should only include the translation, absolutely no other content, and specifically avoid any commentary on the dialogue content.")
      --temperature float32        Temperature setting for the AI. (default 0.7)
      --topp float32               Top_P setting for the AI. (default 0.95)
      --translator string          Specifies the translation service to use, options: 'openai', 'coze', or 'google'. The 'openai' value indicates compatibility with OpenAI-based APIs. (default "google")
      --use2ndsystemprompt         Use 2nd system prompt.
      --use2nduserprompt           Use 2nd user prompt.
      --userprompt string          User prompt provided to the AI, Use '<ot>' as the placeholder in the template to represent the original text to be translated, and '<rt>' to represent the reference translation if any. (default "请将以下台词翻译为中文台词：<ot>")
      --userprompt2 string         User prompt provided to the AI, Use '<ot>' as the placeholder in the template to represent the original text to be translated, and '<rt>' to represent the reference translation if any. (no effect unless use2nduserprompt is true) (default "Please translate the following lines into Chinese: <ot>")
```

## 效果和个人经验 Effects and Personal Experience

如果待翻译的srt文件来自whisper等模型进行的语音转写，由于转写本身总会有错误，进行翻译时会扩大错误（将听错的词翻译为更加离谱的词），用户不应对机翻的结果抱有太大期待。

If the SRT file to be translated comes from voice transcription by models such as whisper, since transcription itself will always have errors, translating it will amplify these mistakes (translating misheard words into even more absurd ones), users should not have too high expectations for machine translation results.

虽然不同模型的翻译水平相差很大，且对于错听词句的纠错能力也有明显差别，但是如果翻译结果非常离谱、完全无法使用，用户应该把工作重心放在调整转写参数、更换转写模型和工具上，与提高翻译质量相比，改变转写质量带来的改善会明显得多。

Although the translation level of different models varies greatly, and their ability to correct misheard words also differs significantly, if the translation results are ridiculously off and completely unusable, users should focus on adjusting transcription parameters, switching transcription models and tools. The improvement from changing transcription quality will be much more noticeable compared to improving translation quality.

### 关于翻译模型（仅供参考）

根据个人不严谨的测试，将转写产生的字幕由日语或英语翻译为中文时：
- 主流模型的最新最强版本（比如`ChatGPT4` `Claude3 Opus` `Gemini Pro 1.5` `Command R+`等等）的翻译水平明显好于google翻译，他们的廉价版本也大多好于google翻译
- 主流开源模型（包括`Llama 3` `Mixtral`等）不一定好于google翻译，但`Qwen 1.5`和`DeepSeek-V2`例外，略强于google翻译，中文语料可能是这些模型的优势
- 为特定任务微调的模型（比如基于qwen微调的[`Sakura-13B-Galgame`](https://github.com/SakuraLLM/Sakura-13B-Galgame)）在调试得当时有出彩的表现
- 需使用恰当的模板（如果本地部署模型进行推理）、提示词（除了提示词本身的内容需要精心调试，提示词本身的语言要和模型训练用的主要语言一致）和其他参数（`temperature`和`top_p`很重要，可参考本工具的默认值自行调试）。

### 我的本地配置（仅供参考）

我本地使用[`Sakura-13B-Galgame`](https://github.com/SakuraLLM/Sakura-13B-Galgame)的配置是这样的：

`ollama`的 Modelfile （注意`TEMPLATE`中的空行也很重要）:
```
FROM ./sakura-14b-qwen2beta-v0.9-Q6_K.gguf

TEMPLATE """<|im_start|>system
{{ .System }}<|im_end|>
<|im_start|>user
{{ .Prompt }}<|im_end|>
<|im_start|>assistant

"""

SYSTEM """你是一个轻小说翻译模型，可以流畅通顺地以日本轻小说的风格将日文翻译成简体中文，并联系上下文正确使用人称代词，不擅自添加原文中没有的代词。"""

PARAMETER mirostat 2
PARAMETER repeat_penalty 1
```

与该模型相关的本工具参数:
```
--temperature=0.1
--topp=0.3
--frequencypenalty=0.1
--userprompt=将下面的日文文本翻译成中文：<ot>
--systemprompt=你是一个轻小说翻译模型，可以流畅通顺地以日本轻小说的风格将日文翻译成简体中文，并联系上下文正确使用人称代词，不擅自添加原文中没有的代词。
```
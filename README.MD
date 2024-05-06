# SubtitleTransGO

一个`srt`字幕文件机翻(translate)/处理(transform)工具，只提供命令行，不适合初级用户。

- 当使用AI翻译时，将字幕一行行分开发给AI，避免超越上下文限制，避免AI拒绝翻译，速度较慢
- 当使用google翻译时，自动以每段不超过5000个字符分切字幕，速度较快
- 支持google翻译（免API）
- 支持OpenAI API（ChatGPT或兼容的API）
- 支持Coze API
- 可选预处理1: 当一个长度为2-4字符之间的词在一行字幕中连续重复出现三次以上，则将其减少为连续重复两次
- 可选预处理2：当一行字幕中只包含一个字符的重复，则将这行字幕删除
- 可选后处理1：当译文的换行数多于原文的换行数，抛弃多出的换行及此后的内容（用于抛弃某些模型——例如Mixtral——自作主张的注释）
- 可在使用AI进行翻译时提供参考译本，提高翻译质量（建议由google翻译生成参考译本）

## 使用之前，从语音转写产生字幕文件

你应该先将待翻译的媒体文件转换为16Khz的wav，有助于提高转写准确性，例如

`ffmpeg -i input.mp4 -vn -acodec pcm_s16le -ar 16000 -ac 2 ./output.wav`

然后你应该使用自己喜欢的转写工具和模型，生成srt文件，例如

`whisperx --model zh-plus/faster-whisper-large-v2-japanese-5k-steps --align_model TKU410410103/wav2vec2-base-japanese-asr --output_format srt --language ja --vad_onset 0.5 --vad_offset 0.363 --chunk_size 9 --no_speech_threshold 0.3 --print_progress True output.wav`

## 如何使用

例如

`stgo ./input.srt --reference=./google.srt --pre1 --pre2 --post1 --translator=openai --apiurl=http://localhost:11434/v1/chat/completions --apikey=xxx --model=qwen:32b-chat-v1.5-q2_K --maxrpm=50 --temperature=0.3 --topp=0.95`

支持环境变量中的`http_proxy`和`https_proxy`设置

在命令行中使用`-h`查看可选参数

```
--apikey string         The access key for the translation API. Not required for the 'google' translator option.
--apiurl string         The URL endpoint for the translation API. Not required for the 'google' translator option.
--bilingual             Enables saving both the original and translated subtitles in the destination SRT file.
--bot string            Identifies the bot to be used, applicable only for 'coze' translator.
--dest string           Path to the destination SRT file for writing.
-h, --help                  help for stgo
--maxrpm int            The maximum number of translation requests permitted per minute. (default 10)
--maxtokens int         The maximum number of tokens contained in each line of translated subtitles. (default 1024)
--model string          Translation model to be used, required only for 'openai' translator.
--post1                 Preprocessing method 1: If the translation has more line breaks than the original text, discard the line break and the content that follows it.
--pre1                  Preprocessing method 1: Reduces any 2 to 4 character long patterns (words) that appear more than twice in subtitles to two occurrences.
--pre2                  Preprocessing method 2: Removes subtitles that consist only of repeated Unicode characters.
--reference string      Path to the SRT file for reference.
--source string         Path to the source SRT file for reading.
--systemprompt string   System prompt provided to the AI. 
--temperature float32   Temperature setting for the AI. (default 0.3)
--topp float32          Top_P setting for the AI. (default 0.95)
--translator string     Specifies the translation service to use, options: 'openai', 'coze', or 'google'. The 'openai' value indicates compatibility with OpenAI-based APIs. (default "google")
--userprompt string     User prompt provided to the AI, Use '<ot>' as the placeholder in the template to represent the original text to be translated, and '<rt>' to represent the reference translation if any. 
```

## 效果

如果待翻译的srt文件来自whisper等模型进行的语音转写，由于转写本身总会有错误，进行翻译时会扩大错误（将听错的词翻译为更加离谱的词），用户不应对机翻的结果抱有太大期待。

根据个人不严谨的测试，将转写产生的字幕由日语/英语翻译为中文时，`ChatGPT4`的翻译水平明显好于google翻译，开源模型（例如`Mixtral-8x7b` `llama3-70b`等）则基本上弱于google翻译，开源模型中`qwen`的成果勉强可以比肩google翻译。没有测试`claude`和`gemini`，拥有条件访问其API的用户可以自行测试。
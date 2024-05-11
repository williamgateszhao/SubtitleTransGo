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

然后你应该使用自己喜欢的转写工具和模型，生成srt文件，例如（注意下面的`vad_threshold`/`vad_onset`/`vad_offset`和`vad_max_speech_duration_s`/`chunk_size`取不同的值，对听写结果的句子长度影响很大，可根据样本自行调试）

`whisper-ctranslate2 --device cuda --model_directory ./whisper-large-v2-japanese --language ja --output_format srt --word_timestamps true --hallucination_silence_threshold 3 --beam_size 5 --vad_filter true --vad_threshold 0.5 --vad_min_silence_duration_ms 500 --vad_max_speech_duration_s 9 --no_speech_threshold 0.3 output.wav`

`whisperx --model zh-plus/faster-whisper-large-v2-japanese-5k-steps --align_model TKU410410103/wav2vec2-base-japanese-asr --output_format srt --language ja --vad_onset 0.5 --vad_offset 0.363 --chunk_size 9 --no_speech_threshold 0.3 --print_progress True output.wav`

## 如何使用

例如

`stgo ./input.srt --reference=./google.srt --pre1 --pre2 --post1 --translator=openai --apiurl=http://localhost:11434/v1/chat/completions --apikey=xxx --model=qwen:32b-chat-v1.5-q2_K --maxrpm=50 --temperature=0.3 --topp=0.95`

支持环境变量中的`http_proxy`和`https_proxy`设置

在命令行中使用`-h`查看可选参数

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
    --maxrpm int                 The maximum number of translation requests permitted per minute. (default 10)
    --maxtokens int              The maximum number of tokens contained in each line of translated subtitles. (default 1024)
    --model string               Translation model to be used, required only for 'openai' translator.
    --post1                      Postprocessing method 1: If the translation has more line breaks than the original text, discard the line break and the content that follows it.
    --pre1                       Preprocessing method 1: Reduces any 2 to 4 character long patterns (words) that appear more than twice in subtitles to two occurrences.
    --pre2                       Preprocessing method 2: Removes subtitles that consist only of repeated Unicode characters.
    --pre3                       Preprocessing method 3: If the duration of a subtitle line is less than 1.2 seconds, extend it to 1.2 seconds, without exceeding the start time of the next subtitle line.
    --reference string           Path to the SRT file for reference.
    --source string              Path to the source SRT file for reading.
    --systemprompt string        System prompt provided to the AI. 
    --temperature float32        Temperature setting for the AI. (default 0.7)
    --topp float32               Top_P setting for the AI. (default 0.95)
    --translator string          Specifies the translation service to use, options: 'openai', 'coze', or 'google'. The 'openai' value indicates compatibility with OpenAI-based APIs. (default "google")
    --userprompt string          User prompt provided to the AI, Use '<ot>' as the placeholder in the template to represent the original text to be translated, and '<rt>' to represent the reference translation if any. 
```

## 效果

如果待翻译的srt文件来自whisper等模型进行的语音转写，由于转写本身总会有错误，进行翻译时会扩大错误（将听错的词翻译为更加离谱的词），用户不应对机翻的结果抱有太大期待。

如果翻译结果非常离谱、完全无法使用，用户应该把工作重心放在调整转写参数、更换转写模型和工具上；与翻译工具相比，带来的改善会明显得多。

根据个人不严谨的测试，将转写产生的字幕由日语或英语翻译为中文时，`ChatGPT4`的翻译水平明显好于google翻译，其他模型不一定好于google翻译，需要自行调试研究，可能需要特定任务微调的模型 （比如基于qwen2微调的[Sakura-13B-Galgame](https://github.com/SakuraLLM/Sakura-13B-Galgame)），并配合恰当的模板和提示词。
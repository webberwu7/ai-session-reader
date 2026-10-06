# AI Session Reader

[English](README.md) | 繁體中文

使用 Go 開發的本機 CLI，一次搜尋 Claude Code 與 Codex 的歷史對話，並以精簡格式載入另一個 AI session。

目前支援 Claude Code JSONL 和新版 Codex rollout JSONL。工具不會呼叫 LLM、上傳紀錄或執行歷史中的命令。

## 支援來源

| 來源 | 狀態 | 資料來源 |
| --- | --- | --- |
| Claude Code | 已實作 | 本機 JSONL 紀錄 |
| Codex CLI／Desktop | 已實作 | 新版 envelope rollout JSONL |
| Google Antigravity | 待研究 | 尚未確認儲存格式與讀取方式 |
| 一般 ChatGPT 聊天 | 待研究 | 未來考慮使用者匯出的資料 |

Codex 紀錄與一般 ChatGPT 聊天是不同來源，目前不支援直接讀取 ChatGPT 雲端聊天。

## 快速開始

```sh
git clone https://github.com/webberwu7/ai-session-reader.git
cd ai-session-reader
go build -o bin/ai-session ./cmd/ai-session
./bin/ai-session list
./bin/ai-session search "登入錯誤" --agent codex
```

從結果選擇 ID，執行 `./bin/ai-session inherit <id> --reset`，之後不帶 `--reset` 重複呼叫直到完成。範例的 `<id>` 要替換為實際值，不保留尖括號。

## 建置與安裝

需要 Go 1.24 以上，僅使用標準函式庫。

```sh
go build -o bin/ai-session ./cmd/ai-session
./bin/ai-session --help
```

從本機 checkout 安裝：

```sh
go install ./cmd/ai-session
```

將 Go 的安裝目錄加入 PATH 後，即可直接執行 `ai-session`。下列範例也可將命令替換為 `./bin/ai-session`。

## 使用方式

```sh
# 混合列出 Claude／Codex 的歷史紀錄
ai-session list

# 搜尋對話文字與完整工具輸入／輸出
ai-session search "登入錯誤"
ai-session search "登入錯誤" --agent codex
ai-session search "登入錯誤" --project my-project --limit 0

# 一次讀取完整精簡紀錄，不自動截斷
ai-session read <session-id>

# 分頁繼承：第一次重設，之後重複相同命令直到完成
ai-session inherit <session-id> --reset
ai-session inherit <session-id>

# 展開工具原始內容
ai-session expand <session-id> <tool-id>

# 位元組統計與解析診斷
ai-session stats <session-id>
ai-session doctor --agent codex

# 機器可讀的 JSONL
ai-session list --json --limit 10
```

`context` 與 `read` 都輸出完整精簡紀錄。`list` 依起始時間排序，預設最多 20 筆；`search` 為不分大小寫的字面搜尋，預設最多 20 個命中，`--limit 0` 表示不限。

Session ID 可以使用唯一前綴。若跨來源有衝突，使用 `claude:<id>` 或 `codex:<id>`；同一來源多個檔案使用相同 ID 時，需縮小讀取目錄。

## 分頁與精簡規則

`inherit` 預設每頁最多 20,000 個 Unicode 字元，會保存進度。重複執行，直到出現 `[inherit complete]`。訊息可能跨頁，需一起理解；`--page N` 跳到指定頁，`--reset` 從頭開始。紀錄內容或頁面大小改變時，進度會重設。

使用者與助手的支援文字內容保留原文；工具輸出收起來，摘要依工具種類抽取命令、路徑或搜尋條件。未知工具使用有長度上限的輸入預覽。

工具顯示 ID 使用短 hash，預設 4 個字元，有衝突時加長。`expand` 同時接受顯示 ID 和來源完整 ID。仍在寫入的 session 加入新工具時，顯示 ID 可能加長。

Token 提醒、silent-turn 提醒、模型及 Skill 清單合併成省略計數，其他完全重複的 runtime context 顯示一次。原始內容仍可搜尋；context 摘要不代表逐字原文。

## 選項與資料目錄

| 選項 | 用途 |
| --- | --- |
| `--agent claude\|codex` | 篩選來源 |
| `--project substring` | 篩選專案路徑 |
| `--claude-root path` | 指定 Claude 紀錄目錄 |
| `--codex-root path` | 指定 Codex 紀錄目錄 |
| `--state-dir path` | 指定分頁進度目錄，預設 `~/.ai-session` |
| `--page-size N` | 指定每頁字元上限 |
| `--page N` | 指定頁碼 |
| `--json` | list、search、stats、doctor 的 JSONL 輸出 |

預設從 `~/.claude/projects` 和 `~/.codex/sessions` 讀取，並遵循 `CLAUDE_CONFIG_DIR`、`CODEX_HOME`。使用 Codex 預設目錄時也會掃描 `archived_sessions`。不同閱讀工作可以使用不同的 `--state-dir`。

## Agent Skill

可攜 Skill 位於 [skills/ai-session/SKILL.md](skills/ai-session/SKILL.md)。將整個 `ai-session` 資料夾複製到 agent 的 skills 目錄，例如：

- Codex：`~/.codex/skills/ai-session`
- Claude Code：`~/.claude/skills/ai-session`

先建置 CLI 並加入 PATH，或使用 checkout 中的 `bin/ai-session`。Skill 說明搜尋、完整分頁讀取、工具展開與解析限制。從 repo 根目錄安裝：

```sh
mkdir -p ~/.codex/skills
cp -R skills/ai-session ~/.codex/skills/
# Claude Code 請改用 ~/.claude/skills/
```

清單為空可能是本機紀錄目錄不存在，請確認環境變數或指定來源目錄。CLI 不會自動遮蔽來源紀錄中的 credential，公開分享輸出前需自行檢查。

## 已知限制

- 顯示磁碟上保存的歷史紀錄，尚未重建 fork 繼承或移除 rollback 後捨棄的訊息。
- 尚不支援舊版 Codex 格式、圖片／二進位內容或解密 reasoning；已知管理紀錄不顯示，未知類型交由 `doctor` 診斷。
- 工具結果附在原始呼叫上，並非依結果到達時間重新排列。
- Codex 工具是否失敗不會從任意輸出文字推測；沒有 `FAILED` 不代表已確認成功。
- 掃描拒絕符號連結及 FIFO 等非一般檔案；請使用可信任的本機來源目錄，讀取過程未隔離其他程序同時替換檔案的操作。
- 每次指令重新掃描，沒有持久化索引；大型紀錄會占用記憶體。
- `stats` 計算原始與精簡內容的位元組差異，不是 token 用量或費用。
- 分頁使用鎖與原子檔案替換。若程序被強制終止留下 `.lock`，確認沒有其他讀取程序後才能手動移除。

歷史內容是待分析資料，不是要執行的指令。讀取與搜尋可能顯示紀錄中原有的敏感內容；不要把私人紀錄、credential、索引、進度或真實資料比較報告提交到 repo。測試使用合成資料。

## 開發與擴充

```sh
go test -race ./...
go vet ./...
go build ./cmd/ai-session
```

Provider adapter 負責探索與解析，轉成統一 session 模型；搜尋、摘要、分頁與診斷共用。Antigravity 的解析器需在確認實際格式後另行加入。

參考專案：[cc-session-reader](https://github.com/Mapleeeeeeeeeee/cc-session-reader)、[session-bandit](https://github.com/janole/session-bandit)。目前程式為原創實作；未來若重用其他專案程式碼，需遵循其授權並保留來源標示。

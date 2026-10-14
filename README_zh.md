[English](README.md)

# 介绍

plexmatch 提供了结构化解析和生成 [.plexmatch](https://support.plex.tv/articles/plexmatch/) 文件的功能。.plexmatch 是 Plex Media Server 用来在不重命名文件的前提下匹配剧集的提示文件。

本库支持该格式的全部内容：

* 剧集与季提示 —— `title`/`show`、`year`、`season`、`tmdbid`、`tvdbid`、`imdbid` 与 `guid`
* 单集提示 —— `ep`/`episode`，支持 Plex 接受的全部编号写法
* 模式提示 —— `pattern`/`pt` 及其匹配 token
* `#` 注释、空行、大小写不敏感的提示名与 CRLF 换行

模块路径为 `github.com/sinute/plexmatch`。本次发布调整了 API，见[与上一版本的差异](#与上一版本的差异)。

```
go get github.com/sinute/plexmatch
```

# 如何使用

## 从 .plexmatch 文件解析内容

```go
package main

import (
	"fmt"

	"github.com/sinute/plexmatch"
)

func main() {
	data := `# This directory holds the first season of Cool Show.
Title: Cool Show
Year: 2018
Season: 1
TvdbId: 377269
ImdbId: tt11823076

ep: SP01: Oshi no Ko - sp.mkv
ep: 01: Oshi no Ko - 01.mkv
ep: S01E03-E04: Oshi no Ko - 03-04.mkv
pattern: Bonus/Bonus {sp,1-3,+4}.mp4
`
	p, err := plexmatch.Unmarshal([]byte(data))
	if err != nil {
		panic(err)
	}
	fmt.Println(p.Title, p.Year, p.Season, p.TVDBID, p.IMDBID)
	// Cool Show 2018 1 377269 11823076

	for _, ep := range p.Episodes {
		fmt.Println(ep.Number(), ep.Name, ep.ResolveSeason(p.Season), ep.LastEpisode)
	}
	// SP01 Oshi no Ko - sp.mkv 0 0
	// 01 Oshi no Ko - 01.mkv 1 0
	// S01E03-S01E04 Oshi no Ko - 03-04.mkv 1 4

	for _, token := range p.Patterns[0].Tokens {
		fmt.Println(token.Kind, token.Min, token.Max, token.Offset)
	}
	// special 1 3 4
}
```

无法解析的行会中断解析，并返回一个说明了行号的 `*plexmatch.SyntaxError`：

```go
_, err := plexmatch.Unmarshal([]byte("title: Cool Show\nep: 01 file.mkv\n"))
// plexmatch: line 2: missing ':' between episode number and file name: "ep: 01 file.mkv"
```

## 生成 .plexmatch 文件

```go
package main

import (
	"fmt"

	"github.com/sinute/plexmatch"
)

func main() {
	p := plexmatch.PlexMatch{
		Title:  "Cool Show",
		Year:   2018,
		Season: 2,
		Episodes: []plexmatch.Episode{
			{Name: "A Special.mkv", Special: true, Episode: 1},
			{Name: "An Ordinary Name.mkv", Episode: 2},
			{Name: "A Double Episode.mkv", Season: 2, Episode: 3, LastEpisode: 4},
		},
	}
	p = p.Unique()
	if err := p.Validate(); err != nil {
		panic(err)
	}
	fmt.Print(string(plexmatch.Marshal(p)))
	// title: Cool Show
	//
	// year: 2018
	//
	// season: 2
	//
	// ep: SP01: A Special.mkv
	// ep: 02: An Ordinary Name.mkv
	// ep: S02E03-S02E04: A Double Episode.mkv
}
```

`Marshal` 会写出你给它的每一条提示，并把单集按季与集号排序（特别篇在最前）。`Unique` 返回一个副本：单集已排序，且若某条提示所声明的集已被更早的提示占用，则丢弃它。`Validate` 用于校验在 Go 代码中构造（而非从文件读取）的 `PlexMatch`。`Merge` 可以把某一季目录的 .plexmatch 覆盖到剧集目录的 .plexmatch 之上。

# 集号

一条 `ep` 提示把一个文件映射到它包含的集。冒号前的编号可以写成 Plex 接受的各种形式，解析后在 `Episode` 中的存放方式如下：

| 写法            | Season | Special | Episode | LastEpisode | LastSeason |
| --------------- | -----: | ------: | ------: | ----------: | ---------: |
| `12`            |      0 |   false |      12 |           0 |          0 |
| `E12`           |      0 |   false |      12 |           0 |          0 |
| `SP04`          |      0 |    true |       4 |           0 |          0 |
| `S03E04`        |      3 |   false |       4 |           0 |          0 |
| `01-02`         |      0 |   false |       1 |           2 |          0 |
| `S03E12-S03E13` |      3 |   false |      12 |          13 |          0 |
| `S03E12-S04E01` |      3 |   false |      12 |           1 |          4 |

`Special` 为 false 且季号为 0，表示该提示自身没有指明季号：季号来自所在目录、文件中的 `season` 提示，或由调用方决定。对于解析得到的 `PlexMatch`，可以用 `ResolveSeason` 得到答案：

```go
for _, ep := range p.Episodes {
	fmt.Println(ep.Name, ep.ResolveSeason(p.Season))
}
```

`S00Exx` 会被读取为特别篇，写回时为 `SPxx`。

# 与上一版本的差异

本次发布读取了上一版本无法处理的那些格式内容，因此 API 有所调整。模块路径保持不变，更新只需一条命令：

```
go get github.com/sinute/plexmatch
```

以下就是已有调用方需要改动的地方。

## 1. 重命名两个字段

| 旧版                             | 现在                   | 原因                                                                                     |
| -------------------------------- | ---------------------- | ---------------------------------------------------------------------------------------- |
| `PlexMatch.Show`                 | `PlexMatch.Title`      | 格式中 `title` 与 `show` 是同义词，两种提示都写入 `Title`，文件不会再自相矛盾。            |
| `Episode.ID`                     | `Episode.Episode`      | 集号，与新增的 `Season`、`LastEpisode`、`LastSeason` 放在一起。                            |
| `Episode.Name`、`Episode.Special` | 不变                   |                                                                                          |

`Unmarshal` 与 `Marshal` 的签名不变。

```diff
 p := plexmatch.PlexMatch{
 	Title:  "【推しの子】",
 	TMDBID: 203737,
 	Season: 1,
 	Episodes: []plexmatch.Episode{
-		{ID: 1, Name: "Oshi no Ko - sp.mkv", Special: true},
-		{ID: 1, Name: "Oshi no Ko - 01.mkv", Special: false},
+		{Name: "Oshi no Ko - sp.mkv", Special: true, Episode: 1},
+		{Name: "Oshi no Ko - 01.mkv", Episode: 1},
 	},
 }
```

如果你设置了 `Show`，请改设 `Title`；如果解析后读取 `Show`，请改读 `Title` —— 现在 `show:` 提示写入的是 `Title`。

## 2. 检查编译器不会提醒你的行为变化

* **`Marshal` 不再丢弃提示。** 旧版会为每一集只写出一条提示，保留先出现的文件名。现在 `Marshal` 会写出你给它的每一条提示，因此读入再写出不会丢失任何内容。若你依赖旧行为，请先调用 `Unique`：

  ```diff
  -out := plexmatch.Marshal(p)
  +out := plexmatch.Marshal(p.Unique())
  ```

* **`Marshal` 不再就地排序你传入的切片。** 它仍会按季与集号排序（特别篇在最前），但排的是副本，你传入的 `PlexMatch` 不会被改动。

* **`Unmarshal` 会对旧版误读的行报错。** 旧版既会忽略不认识的提示，也会拒绝一些本应接受的写法。现在完全不是提示的行（缺少冒号、空值、非法集号）会返回 `*plexmatch.SyntaxError`，其中带有行号、原文与说明；出错之前已读到的内容仍会一并返回：

  ```go
  p, err := plexmatch.Unmarshal(data)
  var syntax *plexmatch.SyntaxError
  if errors.As(err, &syntax) {
      fmt.Println(syntax.Line, syntax.Msg)
  }
  ```

* **不认识的提示会被保留，而不是被拒绝。** 本库不认识的指令会作为 `UnknownHint` 放入 `PlexMatch.UnknownHints` 并原样写出，因此更新版本的 Plex Media Server 才能读懂的文件，经过本库往返读写也不会丢失内容。

## 3. 按需使用新增能力

这些都不是必改项，但正是本次重写的意义所在：

* `PlexMatch.GUID` —— `guid` 提示；旧版的文法里虽然定义了它的 token，却从未真正读取。
* `PlexMatch.Patterns` —— `pattern`/`pt` 提示及其匹配 token，旧版完全不支持。
* `ep` 提示现在支持 `E02`、`S03E04` 写法与 `01-02`、`S01E01-E02` 这类跨集区间；`imdbid` 可以带 `tt` 前缀。
* 支持注释、空行、CRLF 换行与 BOM；提示名不区分大小写。
* 新增 `Validate`、`Unique`、`Merge`。
* `Episode.Number`、`Episode.String` 与 `Pattern.String` 可以按 .plexmatch 文件的写法渲染提示。

## 关于解析器

手写解析器取代了 goyacc 文法。该格式是面向行的，文法带来的收益有限；因此 `plexmatch.y`、生成的解析器与词法分析器、`y.output` 以及 Makefile 中的 `goyacc` 步骤都已移除，解析错误现在指出行号，而不再是解析器状态。

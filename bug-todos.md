# bugs

- 标题中的高亮无法渲染, 如
```
### `char **ss` 与 `char *ss[]`
```

- 下面这种代码高亮无法正常渲染
```
                       MISCELLANEOUS COMMANDS

     -<flag>              Toggle a command line option [see OPTIONS below].
     --<name>             Toggle a command line option, by name.
     _<flag>              Display the setting of a command line option.
     __<name>             Display the setting of an option, by name.
     +cmd                 Execute the less cmd each time a new file is examined.
     !command             Execute the shell command with $SHELL.
     |Xcommand            Pipe file between current pos & mark X to shell command.
     v                    Edit the current file with $VISUAL or $EDITOR.
     V                    Print version number of "less".
```

# todo

- 不通等级的标题自动编号 (✓)

- 添加博客文章的 `最近一次更新时间` (✓)

- 在 pigger 站点主目录下面添加一个 .gitignore 文件和一个 draft 文件夹, 初始化 .gitignore
内容为 draft. 将 draft 文件夹用作一个草稿目录. (✓)

- 在 pigger 站点主目录下面添加一个 home 文件夹, 专门用于存放文档 (✓)

- 制作安装包

- 添加 pigger 版本信息

- 两个相邻的 list 项之间可以加入多个空行以使结构清晰, 比如

```
- item one

- item two

- itme three
```

但是渲染后 item 之间不需要留有空行.

- 一个 list 项的内容中可以有空行隔开两段以使结构化清晰, 比如

```
- A list

    Para one

    Para two

    Para three

        The codes

    Para four
```

渲染时段与段之间需要有一个空行.

- 列表项换行添加新方法: 如果一个列表项后空一行, 那么表示要新换行.
比如

```
- The item

    The content of the item
```

等价于

```
- The item:
    The content of the item
```

都表示在 `The item` 之后新起一行写入 item 的内容.

- 博客系统自动删除 .txt 不存在的文章

- 添加博客文章末尾的版权信息


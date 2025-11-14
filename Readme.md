# About

Swb allows to generate and maintain template based static websites.
Pages content can be written in any choosen language (e.g. markdown),
and a proper HTML document is generated using a shell command based
template.

Two file trees are maintained at the same time:

- A `src` tree that contains all the resources (i.e. images, any kind
of raw file, etc) and webpage content files (e.g. markdown files).
- A `dst` tree that is a (smart) copy of the `src` tree, except that all
content files have been compiled to HTML documents.

The `dst` tree is filled by the swb build process and should no be modified
by hand. The user can add new directory and webpages by creating directory
and content files in the `src` tree, then build the `dst` tree by running
swb.

Each website needs to have a template file that will be used to build every
HTML file of the `dst` tree (for every content file in the `src` tree, swb
runs through the template file and can insert the content in the final HTML
document).

# File trees

## Source

Here is an example of what the `sites` directory can look like. It contains
the `config.json` file that holds the config for both the `example.com` and 
the `zoo.com` websites, a `tpl` directory that contains the template files
for both sites, and finally the `src` dir that the `src` tree of our websites.

```
sites
├── config.json
├── src
│   ├── example.com
│   │   ├── foo
│   │   │   └── index.md
│   │   ├── image.png
│   │   └── index.md
│   └── zoo.com
│       ├── zoo.png
│       └── bar.md
└── tpl
    ├── example.com.tpl
    └── zoo.com.tpl
```

## Destination

The associated `dst` trees (e.g. `/var/www/example.com` and `/var/www/zoo.com`)
will hold the same content, but markdown files have been compiled to HTML document
according the the associated file template.

```
/var/www
├── example.com
│   ├── foo
│   │   └── index.html
│   ├── image.png
│   └── index.html
└── zoo.com
    ├── zoo.png
    └── bar.html
```

# Config

The `config.json` file allows minimal customization and configuration of the
websites building process.

```json
{
    "runCmd": ["bash", "-c"],
    "builder": {
        "ext": ".md",
        "bin": "pandoc"
    },
    "sites": [
        {
            "name": "example.com",
            "srcRoot": "src/example.com",
            "dstRoot": "/var/www/example.com",
            "tplPath": "tpl/example.com.tpl"
        },
        {
            "name": "example.com",
            "srcRoot": "src/zoo.com",
            "dstRoot": "/var/www/zoo.com",
            "tplPath": "tpl/zoo.com.tpl",
            "env": [
                "test=foobar"
            ]
        }
    ]
}
```

## Fields

- `runCmd`: Command that will run the commands in the template files (in the `execvp(3) format with the terminating `NULL`).
- `builder`: The builder is an arbitrary program that can convert any type of file to HTML document (e.g. pandoc).
  * `ext`: File extension of the content files.
  * `bin`: Text that will be stored in the `$builder` env var in template command substitution.
- `sites`: Contains all the websites we want to maintain (HTTP virtual hosts).
  * `name`: Plain name of the website.
  * `srcRoot`: Path of the `src` tree.
  * `dstRoot`: Path of the `dst` tree.
  * `tplPath`: Path of the site's template file.
  * (Optional) `env`: Array of custom environment variables that can be accessed from the template file.

# Templates

Each website must have a template file. A template file is a regular HTML document,
except: lines that begin with `%{` are opening a command substitution, and lines that
begin `}%` are closing a command substitution. Commands are runs as argument of config's
`runCmd` command, and optionaly defined env variable from config's `env` array are
added to the env of the `runCmd` execution, as well as a few built-in ones:

- `$site_name`: Plain website name, as defined in the configuration file.
- `$page_name`: Basename of the HTML document the template is used for, without the `.html` suffix.
- `$src_path`: Absolute path in the `src` tree of the document the template is used for.
- `$dst_path`: Absolute path in the `dst` tree of the document the template is used for.
- `$builder`: Builder command/string, as defined in the configuration file.

## Example

```
<h1>
%{
	echo $page_name
}%
</h1>

Built with <code>swb</code>

%{
	$builder $src_path
}%

Templates can use config defined variables:
%{
	echo $test
}%

This is a footer
```

Note that the `$builder $src_path` command will use the builder command
to convert the markdown file into html and insert it in the template.

# Usage

```
Usage of swb:
  -b    Build the dst trees
  -c string
        Configuration file (default "config.json")
  -k    Clean the dst trees
  -w string
        Working directory (default ".")
```

# Examples

## Build the websites

```
% cd sites
% swb -b
 + /var/www/example.com/
 + /var/www/example.com/foo/
 + /var/www/example.com/foo/index.html
 + /var/www/example.com/image.png
 + /var/www/example.com/index.html
 + /var/www/zoo.com/
 + /var/www/zoo.com/zoo.png
 + /var/www/zoo.com/bar.html
%
% # if we try to rebuild nothing happens 
% swb -b
%
% # if we modify a markdown file, the associated HTML doc will be rebuilt
% echo 'modified !' >>src/zoo.com/bar.md
% swb -b
 ^ /var/www/example.com/index.html
%
% # if we modify the site's template, all HTML doc will be rebuilt
% echo 'modified !' >>tpl/example.com.tpl
% swb -b
 ^ /var/www/example.com/foo/index.html
 ^ /var/www/example.com/index.html
%
% # if we modify non webpage resource, nothing happens, hard link created
% # in the dst tree already reflect the changes.
% echo 'modified !' >>src/zoo.com/zoo.png
% swb -b
%
```
## Clear the websites

```
% cd sites
% # dst trees can be deleted (cleared)
% swb -k
 - /var/www/example.com/*
 - /var/www/zoo.com/*
%
```

## Automatic tidy

```
% cd sites
% swb -b 
 + /var/www/example.com/
 + /var/www/example.com/foo/
 + /var/www/example.com/foo/index.html
 + /var/www/example.com/image.png
 + /var/www/example.com/index.html
 + /var/www/zoo.com/
 + /var/www/zoo.com/zoo.png
 + /var/www/zoo.com/bar.html
%
% # if we remove resources or content file from the src tree,
% # their copy/equivalent is purged from the dst tree.
% rm src/example.com/image.png
% rm src/zoo.com/bar.md
% swb -b
 - /var/www/example.com/image.png
 - /var/www/zoo.com/bar.html
%
```

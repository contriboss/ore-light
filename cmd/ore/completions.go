package main

import (
	"fmt"
)

func printBashCompletion() {
	fmt.Print(`# ore bash completion
_ore_completions() {
    local cur prev commands
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    commands="init add remove update outdated lock fetch install check list show info search why exec clean cache pristine config platform stats help version"

    # Complete commands
    if [ $COMP_CWORD -eq 1 ]; then
        COMPREPLY=( $(compgen -W "${commands}" -- ${cur}) )
        return 0
    fi

    # Complete flags
    case "${prev}" in
        --lockfile|-l)
            COMPREPLY=( $(compgen -f -X '!*.lock' -- ${cur}) )
            ;;
        --vendor)
            COMPREPLY=( $(compgen -d -- ${cur}) )
            ;;
        *)
            COMPREPLY=( $(compgen -W "--help --version --lockfile --vendor --force --verbose" -- ${cur}) )
            ;;
    esac
}

complete -F _ore_completions ore
`)
}

func printZshCompletion() {
	fmt.Print(`#compdef ore
# ore zsh completion

_ore() {
    local -a commands
    commands=(
        'init:Create a new Gemfile'
        'add:Add gems to Gemfile'
        'remove:Remove gems from Gemfile'
        'update:Update gems to their latest versions'
        'outdated:List gems with newer versions available'
        'lock:Regenerate Gemfile.lock from Gemfile'
        'fetch:Download gems into cache (no Ruby required)'
        'install:Install gems from Gemfile.lock'
        'check:Verify all gems are installed'
        'list:List all gems in the current bundle'
        'show:Show the source location of a gem'
        'info:Show detailed information about a gem'
        'search:Search for gems on RubyGems.org'
        'why:Show dependency chains for a gem'
        'exec:Run commands with ore-managed environment'
        'clean:Remove unused gems from vendor directory'
        'cache:Inspect or prune the ore gem cache'
        'pristine:Restore gems to pristine condition (no Ruby required)'
        'config:Get and set Bundler configuration options'
        'platform:Display platform compatibility information'
        'stats:Show Ruby environment statistics'
        'help:Print help information'
        'version:Print version information'
    )

    _arguments -C \
        '(-h --help)'{-h,--help}'[Print help]' \
        '(-V --version)'{-V,--version}'[Print version]' \
        '1: :->command' \
        '*:: :->args'

    case $state in
        command)
            _describe 'ore command' commands
            ;;
        args)
            case $words[1] in
                install|fetch|check|list|lock)
                    _arguments \
                        '--lockfile[Path to Gemfile.lock]:file:_files -g "*.lock"' \
                        '--vendor[Destination directory]:directory:_directories' \
                        '--force[Force reinstall]' \
                        '--verbose[Enable verbose output]'
                    ;;
                add|remove)
                    _arguments \
                        '*:gem name:'
                    ;;
                *)
                    _arguments \
                        '--help[Print command help]'
                    ;;
            esac
            ;;
    esac
}

_ore "$@"
`)
}

func printFishCompletion() {
	fmt.Print(`# ore fish completion

# Commands
complete -c ore -f -n '__fish_use_subcommand' -a 'init' -d 'Create a new Gemfile'
complete -c ore -f -n '__fish_use_subcommand' -a 'add' -d 'Add gems to Gemfile'
complete -c ore -f -n '__fish_use_subcommand' -a 'remove' -d 'Remove gems from Gemfile'
complete -c ore -f -n '__fish_use_subcommand' -a 'update' -d 'Update gems to their latest versions'
complete -c ore -f -n '__fish_use_subcommand' -a 'outdated' -d 'List gems with newer versions available'
complete -c ore -f -n '__fish_use_subcommand' -a 'lock' -d 'Regenerate Gemfile.lock from Gemfile'
complete -c ore -f -n '__fish_use_subcommand' -a 'fetch' -d 'Download gems into cache (no Ruby required)'
complete -c ore -f -n '__fish_use_subcommand' -a 'install' -d 'Install gems from Gemfile.lock'
complete -c ore -f -n '__fish_use_subcommand' -a 'check' -d 'Verify all gems are installed'
complete -c ore -f -n '__fish_use_subcommand' -a 'list' -d 'List all gems in the current bundle'
complete -c ore -f -n '__fish_use_subcommand' -a 'show' -d 'Show the source location of a gem'
complete -c ore -f -n '__fish_use_subcommand' -a 'info' -d 'Show detailed information about a gem'
complete -c ore -f -n '__fish_use_subcommand' -a 'search' -d 'Search for gems on RubyGems.org'
complete -c ore -f -n '__fish_use_subcommand' -a 'why' -d 'Show dependency chains for a gem'
complete -c ore -f -n '__fish_use_subcommand' -a 'exec' -d 'Run commands with ore-managed environment'
complete -c ore -f -n '__fish_use_subcommand' -a 'clean' -d 'Remove unused gems from vendor directory'
complete -c ore -f -n '__fish_use_subcommand' -a 'cache' -d 'Inspect or prune the ore gem cache'
complete -c ore -f -n '__fish_use_subcommand' -a 'pristine' -d 'Restore gems to pristine condition (no Ruby required)'
complete -c ore -f -n '__fish_use_subcommand' -a 'config' -d 'Get and set Bundler configuration options'
complete -c ore -f -n '__fish_use_subcommand' -a 'platform' -d 'Display platform compatibility information'
complete -c ore -f -n '__fish_use_subcommand' -a 'stats' -d 'Show Ruby environment statistics'
complete -c ore -f -n '__fish_use_subcommand' -a 'help' -d 'Print help information'
complete -c ore -f -n '__fish_use_subcommand' -a 'version' -d 'Print version information'

# Global options
complete -c ore -f -s h -l help -d 'Print help'
complete -c ore -f -s V -l version -d 'Print version'

# Common options for install/fetch/check commands
complete -c ore -f -n '__fish_seen_subcommand_from install fetch check list lock' -l lockfile -d 'Path to Gemfile.lock' -r -F
complete -c ore -f -n '__fish_seen_subcommand_from install fetch check list' -l vendor -d 'Destination directory' -r -a '(__fish_complete_directories)'
complete -c ore -f -n '__fish_seen_subcommand_from install' -l force -d 'Force reinstall'
complete -c ore -f -n '__fish_seen_subcommand_from install fetch' -l verbose -d 'Enable verbose output'
`)
}

func runCompletionCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(`usage: ore completion <shell>

Generate shell completion scripts for ore.

Supported shells:
  bash    Generate bash completion script
  zsh     Generate zsh completion script
  fish    Generate fish completion script

Examples:
  # Bash
  ore completion bash > /etc/bash_completion.d/ore
  # or for user-only:
  ore completion bash > ~/.local/share/bash-completion/completions/ore

  # Zsh
  ore completion zsh > /usr/local/share/zsh/site-functions/_ore
  # or add to ~/.zshrc:
  source <(ore completion zsh)

  # Fish
  ore completion fish > ~/.config/fish/completions/ore.fish`)
	}

	shell := args[0]
	switch shell {
	case "bash":
		printBashCompletion()
	case "zsh":
		printZshCompletion()
	case "fish":
		printFishCompletion()
	default:
		return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish)", shell)
	}

	return nil
}

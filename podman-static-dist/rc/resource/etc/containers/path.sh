_c_bin=${XDG_DATA_HOME}/podman-dist/current/usr/local/bin
_c_lib=${XDG_DATA_HOME}/podman-dist/current/usr/local/lib/podman
_c_libexec=${XDG_DATA_HOME}/podman-dist/current/usr/local/libexec/podman
_c_zfunc=${XDG_DATA_HOME}/podman-dist/current/usr/local/share/zsh/site-functions

case ":${PATH}:" in
    *:"$_c_bin":*)
        ;;
    *)
        export PATH="$_c_bin:$PATH"
        ;;
esac


case ":${PATH}:" in
    *:"$_c_lib":*)
        ;;
    *)
        export PATH="$_c_lib:$PATH"
        ;;
esac

case ":${PATH}:" in
    *:"$_c_libexec":*)
        ;;
    *)
        export PATH="$_c_libexec:$PATH"
        ;;
esac

# zsh has no user-level completion dir that compinit scans on its own, so put
# the dist's _podman on fpath directly. bash needs nothing here: install/link
# symlinks the bash completion into ${XDG_DATA_HOME}/bash-completion/completions,
# which bash-completion lazy-loads by itself.
if [ -n "${ZSH_VERSION:-}" ]; then
    case " ${fpath[*]} " in
        *" $_c_zfunc "*)
            ;;
        *)
            fpath=("$_c_zfunc" "${fpath[@]}")
            ;;
    esac
fi

unset _c_bin _c_lib _c_libexec _c_zfunc

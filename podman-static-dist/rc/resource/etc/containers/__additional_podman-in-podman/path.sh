# Should be in sync with ../path.sh
_c_bin=/root/.local/share/podman-dist/current/usr/local/bin
_c_lib=/root/.local/share/podman-dist/current/usr/local/lib/podman
_c_libexec=/root/.local/share/podman-dist/current/usr/local/libexec/podman

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

unset _c_bin _c_lib _c_libexec

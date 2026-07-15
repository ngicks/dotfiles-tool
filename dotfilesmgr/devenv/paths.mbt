///|
/// Path to the CA certificate bundle injected into the build as a secret.
///
/// Honours `SSL_CERT_FILE`; otherwise falls back to the common Debian/Ubuntu
/// location. This is the only filesystem path the devenv build cares about now
/// that image transfer goes through `oci-image-copy` rather than tarballs.
pub fn ca_file_path() -> String {
  @env.get_env_var("SSL_CERT_FILE").unwrap_or(
    // maybe we can later improve this to search 2-3 distro specifc paths.
    "/etc/ssl/certs/ca-certificates.crt",
  )
}

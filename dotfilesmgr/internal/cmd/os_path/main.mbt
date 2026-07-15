///|
fn main {
  try {
    println(@os_path_impl.user_home_dir())
    println(@os_path_impl.user_config_dir())
    println(@os_path_impl.user_cache_dir())
  } catch {
    _ => println("failed")
  }
}

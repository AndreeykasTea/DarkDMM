extern crate core;
extern crate dreammaker as dm;
extern crate serde;
#[macro_use]
extern crate serde_derive;
extern crate serde_json;

use std::ffi::{CStr, CString};
use std::os::raw::c_char;

use environment::parse_environment;
use icon::parse_icon_metadata;

mod environment;
mod icon;

#[no_mangle]
#[allow(non_snake_case)]
pub extern "C" fn SdmmParseEnvironment(native_path: *const c_char) -> *const c_char {
    to_ptr(parse_environment(to_string(native_path)))
}

#[no_mangle]
#[allow(non_snake_case)]
pub extern "C" fn SdmmParseIconMetadata(native_path: *const c_char) -> *const c_char {
    to_ptr(parse_icon_metadata(to_string(native_path)))
}

#[no_mangle]
#[allow(non_snake_case)]
pub extern "C" fn SdmmFreeStr(native_str: *mut c_char) {
    if native_str.is_null() {
        return;
    }
    unsafe { let _ = CString::from_raw(native_str); };
}

/// Convert a native string to a Rust string
fn to_string(pointer: *const c_char) -> String {
    let slice = unsafe { CStr::from_ptr(pointer).to_bytes() };
    // Never panic across the FFI boundary: invalid UTF-8 would abort the whole editor.
    String::from_utf8_lossy(slice).into_owned()
}

/// Convert a Rust string to a native string
fn to_ptr(string: String) -> *const c_char {
    // Interior NUL bytes are not allowed in C strings, so they are stripped instead of panicking.
    let cs = CString::new(string.replace(char::from(0), "")).unwrap_or_default();
    // The ownership is moved to the caller. It must be freed with the SdmmFreeStr.
    cs.into_raw()
}

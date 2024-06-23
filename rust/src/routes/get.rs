use std::io::Read;

use crate::{AppError, AppState, Item};
use axum::{
    extract::{Path, Query, State},
    http::{HeaderValue, StatusCode},
    response::{IntoResponse, Response},
};
// use axum_extra::extract::Query;
use anyhow::anyhow;
use base64::prelude::*;
use foundationdb::{KeySelector, RangeOption};
use serde::Deserialize;
use tracing::{debug, info_span, span, Level};
use validator::Validate;

#[derive(Deserialize, Debug, Validate)]
pub struct GetOrListParams {
    list: Option<String>,

    // List params
    limit: Option<i64>,
    #[serde(default, alias = "vals")]
    with_vals: Option<String>,
    reverse: Option<String>,
    start: Option<i64>,
    end: Option<i64>,
}

pub async fn get_root(
    State(state): State<AppState>,
    Query(params): Query<GetOrListParams>,
) -> Result<Response, AppError> {
    get_or_list_prefix(state, None, &params).await
}

pub async fn get_key(
    State(state): State<AppState>,
    Path(key_prefix): Path<String>,
    Query(params): Query<GetOrListParams>,
) -> Result<Response, AppError> {
    get_or_list_prefix(state, Some(key_prefix), &params).await
}

#[tracing::instrument(level = "debug", skip(state))]
pub async fn get_or_list_prefix(
    state: AppState,
    key_prefix: Option<String>,
    params: &GetOrListParams,
) -> Result<Response, AppError> {
    params.validate()?;

    // Check if we are a list
    match &params.list {
        Some(list) if list.is_empty() => {
            return list_items(state, params, key_prefix).await;
        }
        _ => {}
    }

    if let Some(key) = key_prefix {
        get_item(state, params, &key).await
    } else {
        // Just a health check
        Ok("alive".into_response())
    }
}

#[tracing::instrument(level = "debug", skip(state))]
async fn get_item(
    state: AppState,
    params: &GetOrListParams,
    key: &String,
) -> Result<Response, AppError> {
    let trx = state.fdb.create_trx()?; // no need to commit for read only
    let value = trx.get(key.as_bytes(), true).await?;
    if let Some(val) = value {
        let item: Item = serde_json::from_slice(val.bytes()).unwrap();
        let mut body = item.data;
        if params.start.is_some() || params.end.is_some() {
            // We need to get a subslice of the body
            let start = params.start.or(Some(0)).unwrap() as usize;
            let end = params.end.or(Some(body.len() as i64)).unwrap() as usize;
            debug!(
                "Getting subslice of value for key {} with start={} end={}",
                key, start, end
            );
            body = body[start..end].to_vec();
        }

        Ok(Response::builder()
            .status(StatusCode::OK)
            .header("version", HeaderValue::from(item.version))
            .body(body.into())
            .expect("Failed to construct response"))
    } else {
        Err(AppError::CustomCode(
            anyhow!("not found"),
            StatusCode::NOT_FOUND,
        ))
    }
}

#[tracing::instrument(level = "debug", skip(state))]
async fn list_items(
    state: AppState,
    params: &GetOrListParams,
    prefix: Option<String>,
) -> Result<Response, AppError> {
    let with_vals = params.with_vals.is_some();
    let reverse = params.reverse.is_some();

    // Build the list of items
    let mut range_start = KeySelector::first_greater_or_equal(""); // "" is beginning of DB
    let mut range_end = KeySelector::last_less_or_equal(vec![0xFF]); // 0xFF means end

    if prefix.is_some() && params.reverse.is_some() {
        // Reversing from an end offset
        range_end = KeySelector::last_less_than(prefix.unwrap());
    } else if prefix.is_some() {
        // Forward from a start offset
        range_start = KeySelector::first_greater_than(prefix.unwrap());
    }

    debug!(prefix = prefix, params = params, "Listing items",);

    let opt = RangeOption::from((range_start, range_end));
    opt.reverse = reverse;
    opt.limit = params.limit.or(Some(100));
    let trx = state.fdb.create_trx()?; // no need to commit for read only
    let range = trx.get_range(&opt, 1, true).await?;

    let mut items: Vec<u8> = Vec::new();
    for item in &range {
        items.extend(item.key());
        if with_vals {
            items.extend(b":");
            // Deserialize the data
            let data: Item = serde_json::from_slice(item.value()).unwrap();
            items.extend(BASE64_STANDARD.encode(item.data).as_bytes());
        }
        items.extend(b"\n");
    }

    Ok(items[0..items.len() - b"\n".len()].to_vec().into_response())
}

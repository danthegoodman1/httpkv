use std::{io::Read, time::SystemTime};

use crate::{AppError, AppState, Item};
use axum::{
    body::Bytes,
    extract::{Path, Query, State},
};
// use axum_extra::extract::Query;
use anyhow::anyhow;
use serde::Deserialize;
use tracing::info;
use validator::Validate;

#[derive(Deserialize, Debug, Validate)]
pub struct WriteParams {
    #[serde(default, alias = "ix")]
    if_exists: Option<String>,

    #[serde(default, alias = "nx")]
    not_exists: Option<String>,

    #[serde(default, alias = "v")]
    version: Option<i64>,
}

#[tracing::instrument(level = "debug", skip(state))]
pub async fn write_key(
    Path(key): Path<String>,
    State(state): State<AppState>,
    Query(params): Query<WriteParams>,
    body: Bytes,
) -> Result<String, AppError> {
    let trx = state.fdb.create_trx()?; // no need to commit for read only
    let val = trx.get(key.as_bytes(), true).await?;
    match val {
        Some(val) => {
            let bytes = val.bytes().collect::<Result<Vec<u8>, _>>().unwrap();
            let bytes = bytes.as_slice();
            let item: Item = serde_json::from_slice(bytes).unwrap();
            if let Some(_) = params.not_exists {
                return Err(AppError::CustomCode(
                    anyhow!("Key {} exists (nx)", key),
                    axum::http::StatusCode::CONFLICT,
                ));
            }
            if let Some(version) = params.version {
                if version != item.version {
                    return Err(AppError::CustomCode(
                        anyhow!(
                            "Provided version {} does not match found version {}",
                            version,
                            item.version
                        ),
                        axum::http::StatusCode::CONFLICT,
                    ));
                }
            }
        }
        None => {
            if params.version.is_some() {
                // Fail if version constraint
                return Err(AppError::CustomCode(
                    anyhow!("Key {} does not exist (v)", key,),
                    axum::http::StatusCode::CONFLICT,
                ));
            }
            if params.if_exists.is_some() {
                // Fail if exists constraint
                return Err(AppError::CustomCode(
                    anyhow!("Key {} does not exist (ix)", key,),
                    axum::http::StatusCode::CONFLICT,
                ));
            }
        }
    };

    // Write the value
    let item = crate::Item {
        version: SystemTime::now()
            .duration_since(SystemTime::UNIX_EPOCH)
            .unwrap()
            .as_nanos() as i64,
        data: body.into(),
    };
    let itemBytes = serde_json::to_vec(&item).unwrap();
    trx.set(key.as_bytes(), &itemBytes);

    info!("wrote it");

    trx.commit().await;
    Ok("".to_string())
}

#![no_std]

//! Provider Registry Contract
//!
//! Maintains the active authorization set of providers and attesters.

use soroban_sdk::{contract, contractimpl};

#[contract]
pub struct ProviderRegistryContract;

#[contractimpl]
impl ProviderRegistryContract {}

#![no_std]

//! Care Agreement Contract
//!
//! Enforces the lifecycle and financial settlement of healthcare care agreements.

use soroban_sdk::{contract, contractimpl};

#[contract]
pub struct CareAgreementContract;

#[contractimpl]
impl CareAgreementContract {}

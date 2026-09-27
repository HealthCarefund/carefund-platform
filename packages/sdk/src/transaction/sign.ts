import { Transaction, TransactionBuilder } from "@stellar/stellar-sdk";

/**
 * The signing boundary. This SDK never holds, requests, or derives a
 * private key — a `WalletSigner` is implemented by whatever the browser
 * wallet integration provides (e.g. Freighter, out of scope for this
 * commit) and is the only thing capable of producing a signature. This
 * type intentionally has no "sign with secret key" counterpart.
 */
export interface WalletSigner {
  /** The account address the wallet will sign for. */
  readonly publicKey: string;
  /**
   * Hands the wallet a base64 transaction envelope XDR and the network
   * passphrase it must sign for, and receives back a signed envelope XDR.
   * Implementations must reject signing for the wrong `networkPassphrase`.
   */
  signTransaction(
    transactionXdr: string,
    context: { readonly networkPassphrase: string },
  ): Promise<{ readonly signedTransactionXdr: string }>;
}

/**
 * Sends a prepared transaction to the wallet for signature and parses the
 * signed envelope it returns back into a `Transaction`. This function does
 * not submit anything — it only crosses the signing boundary.
 */
export async function awaitWalletSignature(
  signer: WalletSigner,
  transaction: Transaction,
  networkPassphrase: string,
): Promise<Transaction> {
  const { signedTransactionXdr } = await signer.signTransaction(transaction.toXDR(), {
    networkPassphrase,
  });
  const signed = TransactionBuilder.fromXDR(signedTransactionXdr, networkPassphrase);
  if (!(signed instanceof Transaction)) {
    throw new TypeError("Wallet returned a fee-bump transaction; expected a plain transaction");
  }
  return signed;
}

class BankCatalog {
  /// Map of normalized bank-name alias -> display name.
  /// Ordered longest first to avoid prefix matches (Vietcombank beats Viet).
  static final Map<String, String> aliases = {
    'vietcombank': 'Vietcombank',
    'techcombank': 'Techcombank',
    'vietinbank': 'VietinBank',
    'sacombank': 'Sacombank',
    'agribank': 'Agribank',
    'vpbank': 'VPBank',
    'bidv': 'BIDV',
    'tpbank': 'TPBank',
    'mbbank': 'MB',
    'mb bank': 'MB',
    'acb': 'ACB',
    'ocb': 'OCB',
    'mb': 'MB',
  };
}

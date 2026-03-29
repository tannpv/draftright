export declare enum BillingPeriod {
    NONE = "none",
    MONTHLY = "monthly",
    YEARLY = "yearly"
}
export declare class Plan {
    id: string;
    name: string;
    daily_limit: number;
    price_cents: number;
    billing_period: BillingPeriod;
    is_active: boolean;
    created_at: Date;
    updated_at: Date;
}

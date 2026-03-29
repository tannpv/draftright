export declare enum UserRole {
    USER = "user",
    ADMIN = "admin"
}
export declare class User {
    id: string;
    email: string;
    password_hash: string;
    name: string;
    is_active: boolean;
    role: UserRole;
    created_at: Date;
    updated_at: Date;
}

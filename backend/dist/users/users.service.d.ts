import { Repository } from 'typeorm';
import { User, UserRole } from './entities/user.entity';
export declare class UsersService {
    private readonly usersRepo;
    constructor(usersRepo: Repository<User>);
    findByEmail(email: string): Promise<User | null>;
    findById(id: string): Promise<User | null>;
    create(data: {
        email: string;
        password_hash: string;
        name: string;
        role?: UserRole;
    }): Promise<User>;
    update(id: string, data: Partial<User>): Promise<User>;
    count(): Promise<number>;
    findAll(options: {
        search?: string;
        page?: number;
        limit?: number;
    }): Promise<{
        users: User[];
        total: number;
    }>;
}

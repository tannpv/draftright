import { Repository } from 'typeorm';
import { Plan } from './entities/plan.entity';
export declare class PlansService {
    private readonly plansRepo;
    constructor(plansRepo: Repository<Plan>);
    findAll(): Promise<Plan[]>;
    findById(id: string): Promise<Plan | null>;
    findFreePlan(): Promise<Plan>;
    create(data: Partial<Plan>): Promise<Plan>;
    update(id: string, data: Partial<Plan>): Promise<Plan>;
    softDelete(id: string): Promise<void>;
}

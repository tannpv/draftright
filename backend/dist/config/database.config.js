"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.databaseConfig = void 0;
const databaseConfig = () => ({
    type: 'postgres',
    url: process.env.DATABASE_URL || 'postgresql://draftright:password@localhost:5432/draftright',
    autoLoadEntities: true,
    synchronize: process.env.NODE_ENV !== 'production',
    logging: process.env.NODE_ENV === 'development',
});
exports.databaseConfig = databaseConfig;
//# sourceMappingURL=database.config.js.map
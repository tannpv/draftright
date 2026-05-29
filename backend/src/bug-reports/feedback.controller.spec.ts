import { Test } from '@nestjs/testing';
import { UnauthorizedException } from '@nestjs/common';
import * as jwt from 'jsonwebtoken';
import { FeedbackController } from './feedback.controller';
import { BugReportsService } from './bug-reports.service';

describe('FeedbackController', () => {
  let ctrl: FeedbackController;
  const svc = {
    createFeedback: jest.fn(),
    listPublicFeatures: jest.fn(),
    toggleVote: jest.fn(),
  };
  const SECRET = 'test_secret';

  const reqWith = (token?: string) =>
    ({ headers: token ? { authorization: `Bearer ${token}` } : {} } as any);

  beforeAll(() => { process.env.JWT_SECRET = SECRET; });
  beforeEach(async () => {
    jest.clearAllMocks();
    const mod = await Test.createTestingModule({
      controllers: [FeedbackController],
      providers: [{ provide: BugReportsService, useValue: svc }],
    }).compile();
    ctrl = mod.get(FeedbackController);
  });

  it('POST /feedback stamps user_id from a valid Bearer token', async () => {
    svc.createFeedback.mockResolvedValue({ id: 'r1' });
    const token = jwt.sign({ sub: 'user-9' }, SECRET);
    const out = await ctrl.create({ kind: 'feature', title: 'X', target_platform: 'mac', description: 'd', source: 'web' } as any, reqWith(token));
    expect(svc.createFeedback).toHaveBeenCalledWith(expect.objectContaining({ kind: 'feature' }), 'user-9');
    // display_no isn't on the mock so ref is null and message lacks the ref suffix.
    expect(out).toEqual({ id: 'r1', ref: null, message: 'Feature request received. Thanks!' });
  });

  it('POST /feedback treats a bad token as anonymous', async () => {
    svc.createFeedback.mockResolvedValue({ id: 'r2' });
    await ctrl.create({ kind: 'feature', title: 'X', target_platform: 'mac', description: 'd', source: 'web' } as any, reqWith('garbage'));
    expect(svc.createFeedback).toHaveBeenCalledWith(expect.anything(), null);
  });

  it('GET /feedback passes filters + decoded userId through', async () => {
    svc.listPublicFeatures.mockResolvedValue({ rows: [], total: 0 });
    const token = jwt.sign({ sub: 'user-3' }, SECRET);
    await ctrl.list({ status: 'reviewing', target_platform: 'linux', page: '2', limit: '5' } as any, reqWith(token));
    expect(svc.listPublicFeatures).toHaveBeenCalledWith(
      { status: 'reviewing', target_platform: 'linux', page: '2', limit: '5' }, 'user-3',
    );
  });

  it('POST /feedback/:id/vote requires a valid token', async () => {
    await expect(ctrl.vote('feat-1', reqWith())).rejects.toBeInstanceOf(UnauthorizedException);
    await expect(ctrl.vote('feat-1', reqWith('garbage'))).rejects.toBeInstanceOf(UnauthorizedException);
  });

  it('POST /feedback/:id/vote toggles via the service', async () => {
    svc.toggleVote.mockResolvedValue({ vote_count: 3, hasVoted: true });
    const token = jwt.sign({ sub: 'user-7' }, SECRET);
    const out = await ctrl.vote('feat-1', reqWith(token));
    expect(svc.toggleVote).toHaveBeenCalledWith('feat-1', 'user-7');
    expect(out).toEqual({ vote_count: 3, hasVoted: true });
  });
});

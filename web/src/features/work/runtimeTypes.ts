import type {components} from '../../lib/api/schema';
export type TaskSection='overview'|'records'|'flow';
export type WorkCapabilities=components['schemas']['WorkCapabilities'];
export type ExecutionException=components['schemas']['ExecutionException'];
export type ExceptionWaiver=components['schemas']['ExceptionWaiver'];
export type ExecutionReview=components['schemas']['ExecutionReview'];
export type DiscardReview=components['schemas']['DraftDiscardReview'];
export type WorkEditFields={title:string;description:string;criteria:string;ownerSeatId:string;reviewerSeatId:string;participantIds:string[];requirements:import('./types').Requirement[]};

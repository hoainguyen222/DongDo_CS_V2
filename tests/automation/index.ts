/**
 * ============================================================
 * Automation Tests Index
 * ============================================================
 * Export tất cả các test functions để sử dụng dễ dàng
 */

// Export config and types
export {
  TEST_CONFIG,
  TEST_DATA,
  TestType,
  testLogger,
} from './config';

export type { TestResult } from './config';

// Export agent config
export {
  getAgentConfigManager,
  createAgentConfigManager,
} from './agent-config';

export type {
  AgentUser,
  AgentConfig,
  ServerConfig,
  TestDefaults,
  AIQuestion,
  HumanCSTrigger,
  TestScenario,
  ReportingConfig,
} from './agent-config';

// Export guest load test
export {
  runGuestConcurrencyTest,
  registerGuest,
  testConcurrentGuestRegistration,
} from './guest-load-test';

// Export message load test
export {
  runMessageConcurrencyTest,
  sendMessage,
  getChatHistory,
  waitForAIResponse,
} from './message-load-test';

// Export WebSocket test
export {
  runWebSocketTest,
  TestWSClient,
} from './websocket-test';

// Export AI response test
export {
  runAIResponseTest,
  testSingleQuestion,
  waitForAIResponse as waitForAIResponseFromAI,
} from './ai-response-test';

// Export team agent test
export {
  runTeamAgentTest,
  getCSAgents,
  getOpenCases,
  takeCase,
  sendCSReply,
  resolveCase,
} from './team-agent-test';

// Export batch runner (run separately with: npx ts-node batch-runner.ts)

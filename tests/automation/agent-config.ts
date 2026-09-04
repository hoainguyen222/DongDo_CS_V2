/**
 * ============================================================
 * Agent Config Loader & Token Manager
 * ============================================================
 * Load agent configurations from JSON and manage bearer tokens
 * Support for batch and parallel testing with authenticated users
 */

import * as fs from 'fs';
import * as path from 'path';

// Types
export interface AgentUser {
  id: string;
  username: string;
  password: string;
  full_name: string;
  role: string;
  enabled: boolean;
}

export interface ServerConfig {
  api: string;
  ws: string;
}

export interface TestDefaults {
  max_concurrent_guests: number;
  max_concurrent_messages: number;
  request_timeout_ms: number;
  ai_response_timeout_ms: number;
  message_delay_ms: number;
  max_retries: number;
  retry_delay_ms: number;
}

export interface AIQuestion {
  id: string;
  category: string;
  question: string;
  expected_keywords?: string[];
}

export interface HumanCSTrigger {
  id: string;
  pattern: string;
  expected_action: string;
}

export interface TestScenario {
  guest_count: number;
  messages_per_guest: number;
  ai_questions?: string[];
  concurrent_agents?: number;
  burst_messages?: boolean;
}

export interface ReportingConfig {
  save_reports: boolean;
  report_directory: string;
  report_format: string;
  include_screenshots: boolean;
  console_output: boolean;
  verbose: boolean;
}

export interface AgentConfig {
  name: string;
  version: string;
  description: string;
  servers: {
    local: ServerConfig;
    staging: ServerConfig;
    production: ServerConfig;
  };
  test_defaults: TestDefaults;
  agent_users: AgentUser[];
  test_scenarios: {
    smoke_test: TestScenario;
    light_load: TestScenario;
    medium_load: TestScenario;
    heavy_load: TestScenario;
    stress_test: TestScenario;
  };
  ai_test_questions: AIQuestion[];
  human_cs_triggers: HumanCSTrigger[];
  reporting: ReportingConfig;
}

// Token cache
interface TokenCache {
  [username: string]: {
    token: string;
    expires_at: number;
    user: AgentUser;
  };
}

class AgentConfigManager {
  private config: AgentConfig | null = null;
  private tokenCache: TokenCache = {};
  private configPath: string;

  constructor(configPath?: string) {
    this.configPath = configPath || path.join(__dirname, 'config.agents.json');
  }

  /**
   * Load configuration from JSON file
   */
  loadConfig(): AgentConfig {
    if (this.config) {
      return this.config;
    }

    try {
      const rawData = fs.readFileSync(this.configPath, 'utf-8');
      this.config = JSON.parse(rawData) as AgentConfig;
      return this.config;
    } catch (error) {
      throw new Error(`Failed to load config from ${this.configPath}: ${error}`);
    }
  }

  /**
   * Get configuration
   */
  getConfig(): AgentConfig {
    if (!this.config) {
      return this.loadConfig();
    }
    return this.config;
  }

  /**
   * Get server config by environment
   */
  getServerConfig(env: 'local' | 'staging' | 'production' = 'local'): ServerConfig {
    const config = this.getConfig();
    return config.servers[env];
  }

  /**
   * Get enabled agent users
   */
  getEnabledAgents(): AgentUser[] {
    const config = this.getConfig();
    return config.agent_users.filter(agent => agent.enabled);
  }

  /**
   * Get agent by username
   */
  getAgentByUsername(username: string): AgentUser | undefined {
    const config = this.getConfig();
    return config.agent_users.find(agent => agent.username === username);
  }

  /**
   * Get all agent users
   */
  getAllAgents(): AgentUser[] {
    const config = this.getConfig();
    return config.agent_users;
  }

  /**
   * Get test defaults
   */
  getTestDefaults(): TestDefaults {
    return this.getConfig().test_defaults;
  }

  /**
   * Get AI test questions
   */
  getAIQuestions(): AIQuestion[] {
    return this.getConfig().ai_test_questions;
  }

  /**
   * Get human CS triggers
   */
  getHumanCSTriggers(): HumanCSTrigger[] {
    return this.getConfig().human_cs_triggers;
  }

  /**
   * Get test scenario by name
   */
  getTestScenario(name: string): TestScenario | undefined {
    return this.getConfig().test_scenarios[name as keyof typeof this.getConfig().test_scenarios];
  }

  /**
   * Get reporting config
   */
  getReportingConfig(): ReportingConfig {
    return this.getConfig().reporting;
  }

  /**
   * Login as agent and get bearer token
   */
  async loginAgent(username: string): Promise<{ token: string; user: AgentUser } | null> {
    // Check cache first
    const cached = this.tokenCache[username];
    if (cached && cached.expires_at > Date.now()) {
      return { token: cached.token, user: cached.user };
    }

    const agent = this.getAgentByUsername(username);
    if (!agent) {
      console.error(`Agent ${username} not found`);
      return null;
    }

    const serverConfig = this.getServerConfig();
    
    try {
      const response = await fetch(`${serverConfig.api}/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: agent.username,
          password: agent.password,
        }),
      });

      if (!response.ok) {
        console.error(`Login failed for ${username}: HTTP ${response.status}`);
        return null;
      }

      const data = await response.json();
      const token = data.token || data.access_token;

      if (!token) {
        console.error(`No token in response for ${username}`);
        return null;
      }

      // Cache the token (assume 1 hour expiry)
      this.tokenCache[username] = {
        token,
        expires_at: Date.now() + 3600000, // 1 hour
        user: agent,
      };

      return { token, user: agent };
    } catch (error) {
      console.error(`Login error for ${username}:`, error);
      return null;
    }
  }

  /**
   * Login multiple agents and return their tokens
   */
  async loginAllAgents(): Promise<Map<string, { token: string; user: AgentUser }>> {
    const agents = this.getEnabledAgents();
    const results = new Map<string, { token: string; user: AgentUser }>();

    // Login in parallel for speed
    const promises = agents.map(async (agent) => {
      const result = await this.loginAgent(agent.username);
      if (result) {
        results.set(agent.username, result);
      }
      return result;
    });

    await Promise.all(promises);
    return results;
  }

  /**
   * Login agents for batch testing
   * @param count Number of agents to login
   * @param parallel Run logins in parallel (default: true)
   */
  async loginAgentsForBatch(count: number, parallel: boolean = true): Promise<Array<{ token: string; user: AgentUser }>> {
    const enabledAgents = this.getEnabledAgents();
    const agentsToLogin = enabledAgents.slice(0, Math.min(count, enabledAgents.length));

    if (parallel) {
      // Parallel login
      const results = await Promise.all(
        agentsToLogin.map(agent => this.loginAgent(agent.username))
      );
      return results.filter((r): r is { token: string; user: AgentUser } => r !== null);
    } else {
      // Sequential login
      const results: Array<{ token: string; user: AgentUser }> = [];
      for (const agent of agentsToLogin) {
        const result = await this.loginAgent(agent.username);
        if (result) {
          results.push(result);
        }
      }
      return results;
    }
  }

  /**
   * Make authenticated API request
   */
  async authenticatedRequest(
    username: string,
    endpoint: string,
    options: RequestInit = {}
  ): Promise<Response> {
    const auth = await this.loginAgent(username);
    if (!auth) {
      throw new Error(`Failed to authenticate as ${username}`);
    }

    const serverConfig = this.getServerConfig();
    const headers = {
      ...options.headers,
      'Authorization': `Bearer ${auth.token}`,
      'Content-Type': 'application/json',
    };

    return fetch(`${serverConfig.api}${endpoint}`, {
      ...options,
      headers,
    });
  }

  /**
   * Run batch operations with authenticated agents
   */
  async runBatchOperation<T>(
    operation: (token: string, user: AgentUser) => Promise<T>,
    options: {
      agentCount: number;
      parallel?: boolean;
      onProgress?: (completed: number, total: number) => void;
    }
  ): Promise<{ results: T[]; errors: string[] }> {
    const { agentCount, parallel = true, onProgress } = options;
    
    const agents = await this.loginAgentsForBatch(agentCount, parallel);
    const results: T[] = [];
    const errors: string[] = [];

    if (parallel) {
      // Parallel execution
      const promises = agents.map(async (agent, index) => {
        try {
          const result = await operation(agent.token, agent.user);
          onProgress?.(index + 1, agents.length);
          return result;
        } catch (error: any) {
          errors.push(`Agent ${agent.user.username}: ${error.message}`);
          return null;
        }
      });

      const settled = await Promise.all(promises);
      results.push(...settled.filter((r): r is T => r !== null));
    } else {
      // Sequential execution
      for (let i = 0; i < agents.length; i++) {
        try {
          const result = await operation(agents[i].token, agents[i].user);
          results.push(result);
          onProgress?.(i + 1, agents.length);
        } catch (error: any) {
          errors.push(`Agent ${agents[i].user.username}: ${error.message}`);
        }
      }
    }

    return { results, errors };
  }

  /**
   * Clear token cache
   */
  clearTokenCache(): void {
    this.tokenCache = {};
  }

  /**
   * Reload configuration
   */
  reload(): void {
    this.config = null;
    this.loadConfig();
  }
}

// Singleton instance
let configManager: AgentConfigManager | null = null;

export function getAgentConfigManager(configPath?: string): AgentConfigManager {
  if (!configManager) {
    configManager = new AgentConfigManager(configPath);
  }
  return configManager;
}

// Export factory for creating new instances
export function createAgentConfigManager(configPath?: string): AgentConfigManager {
  return new AgentConfigManager(configPath);
}

// Export types for use
export type { TokenCache };

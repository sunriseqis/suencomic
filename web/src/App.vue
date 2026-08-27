<template>
  <div class="app-container">
    <!-- Bauhaus Header Banner -->
    <header class="bh-header">
      <div class="header-left">
        <div class="bh-logo">
          <span class="shape-circle"></span>
          <span class="shape-square"></span>
          <span class="shape-triangle"></span>
        </div>
        <div class="brand-text">
          <h1 class="main-title">SUENCOMIC</h1>
          <span class="sub-title">MULTI-SOURCE MANGA DOWNLOAD PLATFORM</span>
        </div>
      </div>

      <!-- Top-right single benchmark button -->
      <div class="header-right">
        <button class="bh-btn bh-btn-sm bh-btn-yellow speed-action-btn" :disabled="testingSpeed" @click="runSpeedTest">
          <span v-if="testingSpeed">⚡ 测速中...</span>
          <span v-else>⚡ 源测速 (BENCHMARK)</span>
        </button>
      </div>
    </header>

    <!-- Bauhaus Scrolling Marquee Decorative Ticker Bar (包豪斯流动装饰测速条) -->
    <div class="marquee-ticker-bar">
      <div class="ticker-label mono">
        <span class="ticker-dot"></span>
        STATUS
      </div>
      <div class="marquee-track">
        <div class="marquee-content">
          <span v-for="s in speedResults" :key="'m1_' + s.source_id" class="marquee-item" :class="{ 'is-fastest': s.is_fastest, 'is-offline': !s.available }">
            <span class="m-icon">▲</span>
            <span class="m-name">{{ s.source_name }}</span>
            <span v-if="s.available" class="m-latency mono">{{ s.latency_ms }}ms</span>
            <span v-else class="m-latency offline mono">OFFLINE</span>
            <span v-if="s.is_fastest" class="m-fastest-tag">FASTEST</span>
          </span>
          <span class="marquee-item sep mono">◆ AUTO-FALLBACK: ON</span>
          <span class="marquee-item sep mono">■ OUTPUT: ./download</span>
          <span class="marquee-item sep mono">● FORMATS: PDF · RAW · CBZ · EPUB</span>
          
          <!-- Loop duplicate -->
          <span v-for="s in speedResults" :key="'m2_' + s.source_id" class="marquee-item" :class="{ 'is-fastest': s.is_fastest, 'is-offline': !s.available }">
            <span class="m-icon">▲</span>
            <span class="m-name">{{ s.source_name }}</span>
            <span v-if="s.available" class="m-latency mono">{{ s.latency_ms }}ms</span>
            <span v-else class="m-latency offline mono">OFFLINE</span>
            <span v-if="s.is_fastest" class="m-fastest-tag">FASTEST</span>
          </span>
          <span class="marquee-item sep mono">◆ AUTO-FALLBACK: ON</span>
          <span class="marquee-item sep mono">■ OUTPUT: ./download</span>
          <span class="marquee-item sep mono">● FORMATS: PDF · RAW · CBZ · EPUB</span>
        </div>
      </div>
    </div>

    <!-- Bauhaus Navigation Tabs (Consistent Stacked Format) -->
    <nav class="bh-nav">
      <button 
        class="nav-tab" 
        :class="{ active: currentTab === 'home' }" 
        @click="currentTab = 'home'"
      >
        <span class="tab-idx">01</span>
        <div class="tab-content-wrap">
          <span class="tab-name-zh">热门首页</span>
          <span class="tab-name-en mono">HOME</span>
        </div>
      </button>
      <button 
        class="nav-tab" 
        :class="{ active: currentTab === 'search' }" 
        @click="currentTab = 'search'"
      >
        <span class="tab-idx">02</span>
        <div class="tab-content-wrap">
          <span class="tab-name-zh">漫画探索</span>
          <span class="tab-name-en mono">SEARCH</span>
        </div>
      </button>
      <button 
        class="nav-tab" 
        :class="{ active: currentTab === 'tasks' }" 
        @click="currentTab = 'tasks'"
      >
        <span class="tab-idx">03</span>
        <div class="tab-content-wrap">
          <span class="tab-name-zh">下载队列</span>
          <span class="tab-name-en mono">QUEUE</span>
        </div>
        <span v-if="activeTasksCount > 0" class="tab-badge">{{ activeTasksCount }}</span>
      </button>
      <button 
        class="nav-tab" 
        :class="{ active: currentTab === 'tracker' }" 
        @click="currentTab = 'tracker'"
      >
        <span class="tab-idx">04</span>
        <div class="tab-content-wrap">
          <span class="tab-name-zh">追更订阅</span>
          <span class="tab-name-en mono">TRACKER</span>
        </div>
      </button>
      <button 
        class="nav-tab" 
        :class="{ active: currentTab === 'storage' }" 
        @click="currentTab = 'storage'; loadDownloadsList()"
      >
        <span class="tab-idx">05</span>
        <div class="tab-content-wrap">
          <span class="tab-name-zh">本地书架</span>
          <span class="tab-name-en mono">STORAGE</span>
        </div>
      </button>
      <button 
        class="nav-tab" 
        :class="{ active: currentTab === 'settings' }" 
        @click="currentTab = 'settings'"
      >
        <span class="tab-idx">06</span>
        <div class="tab-content-wrap">
          <span class="tab-name-zh">系统配置</span>
          <span class="tab-name-en mono">CONFIG</span>
        </div>
      </button>
    </nav>

    <!-- MAIN CONTENT AREA -->
    <main class="bh-main">
      <!-- TAB 1: HOME // 首页 & 集英社热门排行 -->
      <section v-if="currentTab === 'home'" class="tab-section">
        <!-- Hero Banner -->
        <div class="home-hero bh-card">
          <div class="hero-text">
            <span class="bh-badge bh-badge-red mono">// SHUEISHA & TOP CHARTS</span>
            <h2 class="hero-title">集英社周刊少年 JUMP 专区 & 全网实时热榜</h2>
            <p class="hero-desc">聚合收录集英社殿堂级名作与全网最新排行漫画，支持多源极速下载、原画质解析与一键导出 PDF/CBZ/EPUB。</p>
          </div>
          <button class="bh-btn bh-btn-primary hero-btn" @click="currentTab = 'search'">
            探索漫画全库
          </button>
        </div>

        <!-- Shueisha Collection Section -->
        <div class="home-section-block">
          <div class="section-title-bar">
            <div class="title-left">
              <span class="block-indicator red-block"></span>
              <h3 class="section-heading">集英社（SHUEISHA / 少年JUMP）殿堂名作榜</h3>
            </div>
            <span class="mono sec-sub">收录 尾田荣一郎 / 吾峠呼世晴 / 芥见下下 / 藤本树 / 岸本齐史 等大师作品</span>
          </div>

          <div class="manga-grid">
            <div 
              v-for="(item, idx) in homeData.shueisha" 
              :key="'shueisha_' + item.id" 
              class="manga-card bh-card"
              @click="openMangaDetail(item)"
            >
              <div class="manga-cover-wrap">
                <img :src="item.cover || 'https://via.placeholder.com/200x280?text=No+Cover'" class="manga-cover" alt="cover" loading="lazy" />
                <span class="rank-badge mono">TOP {{ idx + 1 }}</span>
                <span class="source-tag" :class="'src-' + item.source">{{ item.source_name }}</span>
              </div>
              <div class="manga-info">
                <h4 class="manga-title" :title="item.title">{{ item.title }}</h4>
                <p v-if="item.author" class="manga-author">{{ item.author }}</p>
                <p v-if="item.latest_chapter" class="manga-latest">{{ item.latest_chapter }}</p>
                <button class="bh-btn bh-btn-sm bh-btn-primary full-w-btn">
                  查看下载
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Live Trending Leaderboard -->
        <div v-if="homeData.trending && homeData.trending.length > 0" class="home-section-block">
          <div class="section-title-bar">
            <div class="title-left">
              <span class="block-indicator yellow-block"></span>
              <h3 class="section-heading">全网多源实时热度飙升榜</h3>
            </div>
            <span class="mono sec-sub">实时爬取三大源站点热门检索与人气排行</span>
          </div>

          <div class="manga-grid">
            <div 
              v-for="(item, idx) in homeData.trending" 
              :key="'trending_' + item.source + '_' + item.id" 
              class="manga-card bh-card"
              @click="openMangaDetail(item)"
            >
              <div class="manga-cover-wrap">
                <img :src="item.cover || 'https://via.placeholder.com/200x280?text=No+Cover'" class="manga-cover" alt="cover" loading="lazy" />
                <span class="rank-badge mono" :class="{ 'top-three': idx < 3 }">#{{ idx + 1 }}</span>
                <span class="source-tag" :class="'src-' + item.source">{{ item.source_name }}</span>
              </div>
              <div class="manga-info">
                <h4 class="manga-title" :title="item.title">{{ item.title }}</h4>
                <p v-if="item.author" class="manga-author">作者: {{ item.author }}</p>
                <p v-if="item.latest_chapter" class="manga-latest">{{ item.latest_chapter }}</p>
                <button class="bh-btn bh-btn-sm bh-btn-yellow full-w-btn">
                  查看下载
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- TAB 2: EXPLORE / SEARCH (No search recommendations below input) -->
      <section v-if="currentTab === 'search'" class="tab-section">
        <div class="search-hero bh-card">
          <div class="search-top-bar">
            <span class="mono sec-tag">// 02 UNIFIED MULTI-SOURCE SEARCH</span>
            <div class="source-filter-pills">
              <span 
                class="filter-pill" 
                :class="{ active: searchSource === 'all' }" 
                @click="searchSource = 'all'"
              >ALL SOURCES (聚合四源)</span>
              <span 
                class="filter-pill" 
                :class="{ active: searchSource === 'copymanga' }" 
                @click="searchSource = 'copymanga'"
              >CopyManga (拷贝)</span>
              <span 
                class="filter-pill" 
                :class="{ active: searchSource === 'dm5' }" 
                @click="searchSource = 'dm5'"
              >DM5 (动漫屋)</span>
              <span 
                class="filter-pill" 
                :class="{ active: searchSource === 'mangabz' }" 
                @click="searchSource = 'mangabz'"
              >MangaBZ (漫画BZ)</span>
              <span 
                class="filter-pill" 
                :class="{ active: searchSource === 'pica' }" 
                @click="searchSource = 'pica'"
              >PicAcg (哔咔)</span>
            </div>
          </div>

          <div class="search-input-box">
            <input 
              v-model="searchQuery" 
              type="text" 
              class="bh-input search-field" 
              placeholder="输入漫画名称，例如：海贼王 / 咒术回战 / 火影忍者..."
              @keyup.enter="handleSearch"
            />
            <button class="bh-btn bh-btn-primary search-btn" :disabled="searching" @click="handleSearch">
              <span v-if="searching">搜索中...</span>
              <span v-else>搜索</span>
            </button>
          </div>
        </div>

        <!-- Search Results Grid -->
        <div v-if="searching" class="loading-box bh-card">
          <div class="bh-loader"></div>
          <p class="mono">正在向各大漫画源并发检索漫画数据...</p>
        </div>

        <div v-else-if="hasSearched && searchResults.length === 0" class="empty-box bh-card">
          <p class="empty-title">// 未找到相关漫画</p>
          <p class="empty-desc">请尝试更换漫画别名或切换单源检索。</p>
        </div>

        <div v-else-if="searchResults.length > 0" class="results-container">
          <div class="results-header">
            <h3 class="mono">// 搜索结果 ({{ searchResults.length }})</h3>
            <span class="mono hint">点击卡片即可查看完整章节与选择格式下载</span>
          </div>

          <div class="manga-grid">
            <div 
              v-for="item in searchResults" 
              :key="item.source + '_' + item.id" 
              class="manga-card bh-card"
              @click="openMangaDetail(item)"
            >
              <div class="manga-cover-wrap">
                <img :src="item.cover || 'https://via.placeholder.com/200x280?text=No+Cover'" class="manga-cover" alt="cover" loading="lazy" />
                <span class="source-tag" :class="'src-' + item.source">{{ item.source_name }}</span>
                <span v-if="item.latency_ms > 0" class="latency-tag mono">{{ item.latency_ms }}ms</span>
              </div>
              <div class="manga-info">
                <h4 class="manga-title" :title="item.title">{{ item.title }}</h4>
                <p v-if="item.author" class="manga-author">作者: {{ item.author }}</p>
                <p v-if="item.latest_chapter" class="manga-latest">{{ item.latest_chapter }}</p>
                <button class="bh-btn bh-btn-sm bh-btn-primary full-w-btn">
                  查看章节
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- TAB 3: DOWNLOAD QUEUE & MONITOR -->
      <section v-if="currentTab === 'tasks'" class="tab-section">
        <div class="queue-header bh-card">
          <div class="queue-stats">
            <div class="stat-item">
              <span class="stat-num">{{ tasks.length }}</span>
              <span class="stat-lbl mono">TOTAL TASKS</span>
            </div>
            <div class="stat-item active-stat">
              <span class="stat-num">{{ activeTasksCount }}</span>
              <span class="stat-lbl mono">DOWNLOADING</span>
            </div>
            <div class="stat-item completed-stat">
              <span class="stat-num">{{ completedTasksCount }}</span>
              <span class="stat-lbl mono">COMPLETED</span>
            </div>
            <div class="stat-item failed-stat">
              <span class="stat-num">{{ failedTasksCount }}</span>
              <span class="stat-lbl mono">FAILED</span>
            </div>
          </div>

          <div class="queue-actions">
            <button class="bh-btn bh-btn-sm bh-btn-yellow" @click="loadTasks">
              刷新列表
            </button>
            <button class="bh-btn bh-btn-sm bh-btn-black" @click="clearFinishedTasks">
              清理已完成
            </button>
          </div>
        </div>

        <div v-if="tasks.length === 0" class="empty-box bh-card">
          <p class="empty-title">// 下载队列为空</p>
          <p class="empty-desc">当前没有正在下载的任务，请在首页或搜索中选择漫画与章节进行下载。</p>
        </div>

        <div v-else class="tasks-list">
          <div 
            v-for="task in tasks" 
            :key="task.id" 
            class="task-card bh-card"
            :class="'status-' + task.status"
          >
            <div class="task-top">
              <div class="task-title-group">
                <span class="format-badge mono" :class="'fmt-' + task.format">{{ task.format.toUpperCase() }}</span>
                <h4 class="task-manga">{{ task.manga_title }}</h4>
                <span class="task-chapter">// {{ task.chapter_title }}</span>
              </div>
              <div class="task-badges">
                <span class="bh-badge" :class="'status-badge-' + task.status">{{ task.status.toUpperCase() }}</span>
                <span class="bh-badge bh-badge-blue" v-if="task.active_source">SOURCE: {{ task.active_source.toUpperCase() }}</span>
                <span v-if="task.source !== task.active_source" class="bh-badge bh-badge-yellow">AUTOSWITCHED</span>
              </div>
            </div>

            <!-- Progress Bar -->
            <div class="task-progress-wrap">
              <div class="bh-progress-bg">
                <div 
                  class="bh-progress-bar" 
                  :class="{ active: task.status === 'downloading' }" 
                  :style="{ width: task.progress + '%' }"
                ></div>
              </div>
              <div class="progress-labels mono">
                <span>{{ task.downloaded_images }} / {{ task.total_images }} IMAGES</span>
                <span class="progress-pct">{{ Math.round(task.progress) }}%</span>
              </div>
            </div>

            <!-- Output Path if completed -->
            <div v-if="task.status === 'completed'" class="completed-path mono">
              <span>OUTPUT: {{ task.output_path }}</span>
            </div>

            <!-- Error message if failed -->
            <div v-if="task.status === 'failed'" class="error-banner mono">
              <span>ERROR: {{ task.error }}</span>
            </div>

            <!-- Collapsible Logs -->
            <div class="task-logs-container">
              <div class="logs-header" @click="task.showLogs = !task.showLogs">
                <span class="mono logs-title">
                  {{ task.showLogs ? '▼ 收起日志' : '▶ 查看日志 & 换源记录 (' + (task.logs ? task.logs.length : 0) + ')' }}
                </span>
                <span v-if="task.logs && task.logs.length > 0" class="latest-log-snippet mono">
                  {{ task.logs[task.logs.length - 1] }}
                </span>
              </div>
              <div v-if="task.showLogs" class="logs-body mono">
                <div v-for="(log, lIdx) in task.logs" :key="lIdx" class="log-line">
                  {{ log }}
                </div>
              </div>
            </div>

            <!-- Task Controls -->
            <div class="task-controls">
              <button 
                v-if="task.status === 'downloading' || task.status === 'pending'" 
                class="bh-btn bh-btn-sm"
                @click="pauseTask(task.id)"
              >暂停</button>
              <button 
                v-if="task.status === 'paused'" 
                class="bh-btn bh-btn-sm bh-btn-yellow"
                @click="resumeTask(task.id)"
              >继续</button>
              <button 
                v-if="task.status === 'failed'" 
                class="bh-btn bh-btn-sm bh-btn-primary"
                @click="retryTask(task.id)"
              >重试</button>
              <button 
                class="bh-btn bh-btn-sm"
                @click="deleteTask(task.id)"
              >删除</button>
            </div>
          </div>
        </div>
      </section>

      <!-- TAB 4: TRACKER / SUBSCRIPTIONS -->
      <section v-if="currentTab === 'tracker'" class="tab-section">
        <div class="tracker-header bh-card">
          <div>
            <h3 class="mono">// 漫画追更与定时检测</h3>
            <p class="mono hint">订阅漫画后，后台将定时检测最新章节，并根据配置自动推送到下载队列。</p>
          </div>
          <button class="bh-btn bh-btn-primary" :disabled="checkingUpdates" @click="checkUpdatesNow">
            <span v-if="checkingUpdates">检查中...</span>
            <span v-else>立即检查更新</span>
          </button>
        </div>

        <div v-if="subscriptions.length === 0" class="empty-box bh-card">
          <p class="empty-title">// 暂无追更订阅</p>
          <p class="empty-desc">在漫画详情页面中点击「加入追更」即可订阅该漫画。</p>
        </div>

        <div v-else class="subscriptions-grid">
          <div v-for="sub in subscriptions" :key="sub.id" class="sub-card bh-card">
            <div class="sub-info">
              <span class="bh-badge bh-badge-blue">SOURCE: {{ sub.source.toUpperCase() }}</span>
              <h4 class="sub-title">{{ sub.manga_title }}</h4>
              <p class="mono sub-latest">LAST CHAPTER: {{ sub.last_chapter_title || 'N/A' }}</p>
              <p class="mono sub-format">FORMAT: {{ sub.format.toUpperCase() }}</p>
            </div>
            <div class="sub-actions">
              <label class="auto-dl-toggle mono">
                <input type="checkbox" v-model="sub.auto_download" @change="saveSubscription(sub)" />
                自动下载更新章节
              </label>
              <button class="bh-btn bh-btn-sm bh-btn-black" @click="deleteSubscription(sub.id)">
                取消订阅
              </button>
            </div>
          </div>
        </div>
      </section>

      <!-- TAB 5: STORAGE / LOCAL FILES -->
      <section v-if="currentTab === 'storage'" class="tab-section">
        <div class="storage-header bh-card">
          <div>
            <h3 class="mono">// 本地输出目录: ./download</h3>
            <p class="mono hint">已下载的漫画文件存储在固定的 ./download 目录中，支持原图、PDF、CBZ、EPUB 格式。</p>
          </div>
          <button class="bh-btn bh-btn-yellow" @click="loadDownloadsList">
            刷新文件列表
          </button>
        </div>

        <div v-if="downloadFiles.length === 0" class="empty-box bh-card">
          <p class="empty-title">// 暂无下载文件</p>
          <p class="empty-desc">当前 ./download 目录下暂无文件，下载完成后的漫画将在此显示。</p>
        </div>

        <div v-else class="files-table-wrap bh-card">
          <table class="files-table mono">
            <thead>
              <tr>
                <th>文件名</th>
                <th>类型</th>
                <th>大小</th>
                <th>时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="f in downloadFiles" :key="f.path">
                <td class="file-name-cell">
                  <span v-if="f.is_dir">📁</span>
                  <span v-else>📄</span>
                  {{ f.name }}
                </td>
                <td>{{ f.is_dir ? 'DIRECTORY' : 'FILE' }}</td>
                <td>{{ f.is_dir ? '-' : formatBytes(f.size) }}</td>
                <td>{{ f.mod_time }}</td>
                <td>
                  <a 
                    v-if="!f.is_dir" 
                    :href="'/api/downloads/file?path=' + encodeURIComponent(f.path)" 
                    target="_blank" 
                    class="bh-btn bh-btn-sm bh-btn-blue"
                  >
                    下载 / 打开
                  </a>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <!-- TAB 6: SETTINGS -->
      <section v-if="currentTab === 'settings'" class="tab-section">
        <div class="settings-box bh-card">
          <h3 class="mono">// 系统配置 (CONFIGURATION)</h3>
          
          <div class="form-group">
            <label class="form-label mono">HTTP / SOCKS5 PROXY (代理设置):</label>
            <input 
              v-model="appConfig.proxy" 
              type="text" 
              class="bh-input" 
              placeholder="例如: socks5://127.0.0.1:7890 或 http://127.0.0.1:7890" 
            />
            <p class="form-hint mono">配置后将用于所有漫画源爬取与图片下载，支持 HTTP / HTTPS / SOCKS5 代理。</p>
          </div>

          <div class="form-grid-2">
            <div class="form-group">
              <label class="form-label mono">PICACG ACCOUNT (哔咔漫画账号/邮箱):</label>
              <input v-model="appConfig.pica_account" type="text" class="bh-input" placeholder="输入哔咔注册邮箱" />
            </div>
            <div class="form-group">
              <label class="form-label mono">PICACG PASSWORD (哔咔漫画密码):</label>
              <input v-model="appConfig.pica_password" type="password" class="bh-input" placeholder="输入哔咔登录密码" />
            </div>
          </div>

          <div class="form-grid-2">
            <div class="form-group">
              <label class="form-label mono">MAX CONCURRENT CHAPTERS (最大并发话数):</label>
              <input v-model.number="appConfig.max_concurrent_chapters" type="number" min="1" max="10" class="bh-input" />
            </div>
            <div class="form-group">
              <label class="form-label mono">MAX CONCURRENT IMAGES (单话最大图片并发数):</label>
              <input v-model.number="appConfig.max_concurrent_images" type="number" min="1" max="20" class="bh-input" />
            </div>
          </div>

          <div class="form-grid-2">
            <div class="form-group">
              <label class="form-label mono">DEFAULT PACKAGING FORMAT (默认输出格式):</label>
              <select v-model="appConfig.default_format" class="bh-input bh-select">
                <option value="pdf">PDF (合并单本高清文档)</option>
                <option value="raw">RAW (原始单张图片)</option>
                <option value="cbz">CBZ (漫画标准压缩包)</option>
                <option value="epub">EPUB (标准电子书)</option>
              </select>
            </div>
            <div class="form-group">
              <label class="form-label mono">AUTO CHECK INTERVAL (追更检查间隔/分钟):</label>
              <input v-model.number="appConfig.check_interval_minutes" type="number" min="5" max="1440" class="bh-input" />
            </div>
          </div>

          <div class="form-group">
            <label class="auto-dl-toggle mono">
              <input type="checkbox" v-model="appConfig.auto_fallback" />
              启用多源自动测速与智能故障换源 (Smart Auto-Fallback)
            </label>
          </div>

          <div class="form-group">
            <label class="form-label mono">OUTPUT DIRECTORY (固定的输出路径):</label>
            <input :value="appConfig.download_dir" type="text" class="bh-input" disabled />
          </div>

          <div class="settings-actions">
            <button class="bh-btn bh-btn-primary" :disabled="savingConfig" @click="saveAppConfig">
              <span v-if="savingConfig">保存中...</span>
              <span v-else>保存系统配置</span>
            </button>
          </div>
        </div>
      </section>
    </main>

    <!-- MANGA DETAIL & CHAPTER MODAL -->
    <div v-if="showDetailModal" class="modal-overlay" @click.self="showDetailModal = false">
      <div class="modal-content bh-card">
        <div class="modal-header">
          <div class="modal-title-wrap">
            <span class="bh-badge bh-badge-blue">{{ activeManga?.source_name }}</span>
            <h2 class="modal-title">{{ activeManga?.title }}</h2>
          </div>
          <button class="modal-close-btn" @click="showDetailModal = false">✕</button>
        </div>

        <div v-if="loadingDetail" class="loading-box">
          <div class="bh-loader"></div>
          <p class="mono">正在解析漫画章节目录...</p>
        </div>

        <div v-else-if="activeMangaDetail" class="modal-body">
          <div class="detail-top-card">
            <img :src="activeMangaDetail.cover || activeManga?.cover" class="detail-cover" alt="cover" />
            <div class="detail-meta">
              <p v-if="activeMangaDetail.author" class="meta-row"><strong>作者:</strong> {{ activeMangaDetail.author }}</p>
              <p v-if="activeMangaDetail.description" class="meta-desc">{{ activeMangaDetail.description }}</p>
              <p class="meta-chapters-count mono">
                共 <strong>{{ activeMangaDetail.chapters.length }}</strong> 个章节
                <span v-if="activeMangaDetail.chapters.length > 0" class="latest-ch-badge">
                  // 最新: {{ activeMangaDetail.chapters[activeMangaDetail.chapters.length - 1]?.title }}
                </span>
              </p>

              <!-- Track manga button -->
              <button class="bh-btn bh-btn-sm bh-btn-yellow" @click="subscribeCurrentManga">
                + 加入追更订阅
              </button>
            </div>
          </div>

          <!-- Empty Chapters Alert & Auto Switch -->
          <div v-if="activeMangaDetail.chapters.length === 0" class="empty-chapters-card bh-card">
            <div class="empty-alert-icon">⚠️</div>
            <div class="empty-alert-content">
              <h4 class="mono">当前源 [{{ activeMangaDetail.source_name }}] 暂无可下载章节</h4>
              <p class="empty-alert-desc">该作品在 {{ activeMangaDetail.source_name }} 上可能因版权保护或源站变动未提供章节。点击下方按钮即可一键在其他源（如 MangaBZ / CopyManga）中检索！</p>
              <button class="bh-btn bh-btn-primary" @click="searchAlternativeSources(activeMangaDetail.title)">
                ⚡ 在其他漫画源中检索《{{ activeMangaDetail.title }}》
              </button>
            </div>
          </div>

          <!-- Classification Group Tabs (连载单话 / 单行本 / 番外特别篇 / 全部) -->
          <div v-if="categoryTabs.length > 1" class="chapter-group-tabs">
            <button 
              v-for="tab in categoryTabs" 
              :key="tab.key"
              class="grp-tab-btn" 
              :class="{ active: selectedGroup === tab.key }" 
              @click="selectedGroup = tab.key"
            >
              {{ tab.label }} <span class="grp-count">({{ tab.count }})</span>
            </button>
          </div>

          <!-- Format Picker & Selection toolbar -->
          <div class="chapter-toolbar">
            <div class="format-picker">
              <span class="mono toolbar-lbl">OUTPUT:</span>
              <div class="format-tabs">
                <button 
                  class="fmt-btn" 
                  :class="{ active: selectedFormat === 'pdf' }" 
                  @click="selectedFormat = 'pdf'"
                >PDF (合并)</button>
                <button 
                  class="fmt-btn" 
                  :class="{ active: selectedFormat === 'raw' }" 
                  @click="selectedFormat = 'raw'"
                >RAW (原图)</button>
                <button 
                  class="fmt-btn" 
                  :class="{ active: selectedFormat === 'cbz' }" 
                  @click="selectedFormat = 'cbz'"
                >CBZ (压缩包)</button>
                <button 
                  class="fmt-btn" 
                  :class="{ active: selectedFormat === 'epub' }" 
                  @click="selectedFormat = 'epub'"
                >EPUB (电子书)</button>
              </div>
            </div>

            <div class="select-tools">
              <button class="bh-btn bh-btn-sm" @click="sortOrder = (sortOrder === 'desc' ? 'asc' : 'desc')">
                {{ sortOrder === 'desc' ? '最新在前 ⬇' : '正序排列 ⬆' }}
              </button>
              <button class="bh-btn bh-btn-sm" @click="selectAllChapters">全选当前分类</button>
              <button class="bh-btn bh-btn-sm" @click="deselectAllChapters">清空当前分类</button>
              <button class="bh-btn bh-btn-sm" @click="selectRecentChapters(20)">最新20话/卷</button>
            </div>
          </div>

          <!-- Filter Search in modal -->
          <div class="chapter-filter-bar">
            <input 
              v-model="chapterFilter" 
              type="text" 
              class="bh-input ch-filter-input mono" 
              placeholder="快速过滤章节/卷数，例如输入 129 或 第1卷..." 
            />
            <span class="mono ch-filter-count">显示 {{ displayedChapters.length }} / {{ activeMangaDetail.chapters.length }} 项</span>
          </div>

          <!-- Chapter Grid -->
          <div class="chapters-grid-scroll">
            <div class="chapters-grid">
              <div 
                v-for="ch in displayedChapters" 
                :key="ch.id" 
                class="chapter-tile"
                :class="{ 
                  selected: selectedChapterIDs.has(ch.id), 
                  'is-volume-tile': ch.type === 'volume',
                  'is-trial-tile': ch.is_trial || ch.title.includes('试看') || ch.title.includes('試看') 
                }"
                @click="toggleChapterSelection(ch.id)"
              >
                <span class="ch-check">{{ selectedChapterIDs.has(ch.id) ? '✓' : '' }}</span>
                <span class="ch-name" :title="ch.title">{{ ch.title }}</span>
                
                <span v-if="ch.type === 'volume'" class="ch-vol-tag">单行本</span>
                <span v-else-if="ch.type === 'extra'" class="ch-extra-tag">番外</span>

                <span v-if="ch.is_trial || ch.title.includes('试看') || ch.title.includes('試看')" class="ch-trial-tag">试看</span>
                <span v-else class="ch-full-tag">完整</span>
              </div>
            </div>
          </div>

          <!-- Download Action Bar -->
          <div class="modal-footer">
            <div class="footer-info mono">
              已选 <strong class="selected-count">{{ selectedChapterIDs.size }}</strong> / {{ activeMangaDetail.chapters.length }} 话
              <span class="auto-fallback-note">（遇到试看版/限流将自动切换至 CopyManga 等全本源）</span>
            </div>
            <button 
              class="bh-btn bh-btn-primary download-action-btn"
              :disabled="selectedChapterIDs.size === 0 || creatingTasks"
              @click="submitDownloadTasks"
            >
              <span v-if="creatingTasks">创建任务中...</span>
              <span v-else>开始下载 ({{ selectedChapterIDs.size }})</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'

const currentTab = ref('home')
const homeData = ref({
  shueisha: [],
  trending: [],
  classics: []
})
const loadingHome = ref(false)

const searchQuery = ref('')
const searchSource = ref('all')
const searching = ref(false)
const hasSearched = ref(false)
const searchResults = ref([])

const speedResults = ref([])
const testingSpeed = ref(false)

const tasks = ref([])
const subscriptions = ref([])
const downloadFiles = ref([])
const checkingUpdates = ref(false)
const savingConfig = ref(false)

const appConfig = ref({
  download_dir: './download',
  proxy: '',
  max_concurrent_chapters: 3,
  max_concurrent_images: 5,
  default_format: 'pdf',
  check_interval_minutes: 60,
  auto_fallback: true,
  pica_account: '',
  pica_password: ''
})

// Modal states
const showDetailModal = ref(false)
const activeManga = ref(null)
const activeMangaDetail = ref(null)
const loadingDetail = ref(false)
const selectedChapterIDs = ref(new Set())
const selectedFormat = ref('pdf')
const creatingTasks = ref(false)
const chapterFilter = ref('')
const sortOrder = ref('desc')
const selectedGroup = ref('all')

let sseSource = null

// Computed
const activeTasksCount = computed(() => tasks.value.filter(t => t.status === 'downloading' || t.status === 'pending').length)
const completedTasksCount = computed(() => tasks.value.filter(t => t.status === 'completed').length)
const failedTasksCount = computed(() => tasks.value.filter(t => t.status === 'failed').length)

const categoryTabs = computed(() => {
  if (!activeMangaDetail.value?.chapters) return []
  const all = activeMangaDetail.value.chapters
  const chapters = all.filter(c => c.type === 'chapter')
  const volumes = all.filter(c => c.type === 'volume')
  const extras = all.filter(c => c.type === 'extra')

  const tabs = [
    { key: 'all', label: '全部 ALL', count: all.length }
  ]
  if (chapters.length > 0) {
    tabs.push({ key: 'chapter', label: '📌 连载单话', count: chapters.length })
  }
  if (volumes.length > 0) {
    tabs.push({ key: 'volume', label: '📚 单行本卷', count: volumes.length })
  }
  if (extras.length > 0) {
    tabs.push({ key: 'extra', label: '🎁 番外特别篇', count: extras.length })
  }
  return tabs
})

const displayedChapters = computed(() => {
  if (!activeMangaDetail.value?.chapters) return []
  let list = [...activeMangaDetail.value.chapters]
  
  if (selectedGroup.value !== 'all') {
    list = list.filter(c => c.type === selectedGroup.value)
  }

  if (sortOrder.value === 'desc') {
    list.reverse()
  }
  if (chapterFilter.value.trim()) {
    const q = chapterFilter.value.trim().toLowerCase()
    list = list.filter(c => c.title.toLowerCase().includes(q))
  }
  return list
})

// Methods
async function loadHomeData() {
  loadingHome.value = true
  try {
    const res = await fetch('/api/home')
    const json = await res.json()
    if (json.code === 0 && json.data) {
      homeData.value = json.data
    }
  } catch (err) {
    console.error('Load home error:', err)
  } finally {
    loadingHome.value = false
  }
}

async function runSpeedTest() {
  testingSpeed.value = true
  try {
    const res = await fetch('/api/sources/speedtest')
    const json = await res.json()
    if (json.code === 0) {
      speedResults.value = json.data
    }
  } catch (err) {
    console.error('Speed test failed:', err)
  } finally {
    testingSpeed.value = false
  }
}

async function handleSearch() {
  if (!searchQuery.value.trim()) return
  searching.value = true
  hasSearched.value = true
  searchResults.value = []

  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), 8000)

  try {
    const url = `/api/search?q=${encodeURIComponent(searchQuery.value.trim())}&source=${searchSource.value}`
    const res = await fetch(url, { signal: controller.signal })
    const json = await res.json()
    if (json.code === 0 && json.data) {
      searchResults.value = json.data
    }
  } catch (err) {
    if (err.name === 'AbortError') {
      console.warn('Search timed out after 8s')
    } else {
      console.error('Search error:', err)
    }
  } finally {
    clearTimeout(timeoutId)
    searching.value = false
  }
}

async function openMangaDetail(manga) {
  activeManga.value = manga
  activeMangaDetail.value = null
  loadingDetail.value = true
  selectedGroup.value = 'all'
  chapterFilter.value = ''
  selectedChapterIDs.value = new Set()
  selectedFormat.value = appConfig.value.default_format || 'pdf'
  showDetailModal.value = true

  try {
    const res = await fetch(`/api/manga/detail?source=${manga.source}&id=${encodeURIComponent(manga.id)}`)
    const json = await res.json()
    if (json.code === 0 && json.data) {
      activeMangaDetail.value = json.data
      const ids = new Set()
      json.data.chapters.forEach(c => ids.add(c.id))
      selectedChapterIDs.value = ids
    }
  } catch (err) {
    console.error('Detail error:', err)
  } finally {
    loadingDetail.value = false
  }
}

function searchAlternativeSources(title) {
  showDetailModal.value = false
  currentTab.value = 'search'
  searchQuery.value = title
  searchSource.value = 'all'
  handleSearch()
}

function toggleChapterSelection(id) {
  if (selectedChapterIDs.value.has(id)) {
    selectedChapterIDs.value.delete(id)
  } else {
    selectedChapterIDs.value.add(id)
  }
  selectedChapterIDs.value = new Set(selectedChapterIDs.value)
}

function selectAllChapters() {
  const ids = new Set(selectedChapterIDs.value)
  displayedChapters.value.forEach(c => ids.add(c.id))
  selectedChapterIDs.value = ids
}

function deselectAllChapters() {
  const ids = new Set(selectedChapterIDs.value)
  displayedChapters.value.forEach(c => ids.delete(c.id))
  selectedChapterIDs.value = ids
}

function selectRecentChapters(count) {
  const ids = new Set(selectedChapterIDs.value)
  const list = displayedChapters.value.slice(0, count)
  list.forEach(c => ids.add(c.id))
  selectedChapterIDs.value = ids
}

function selectLatestChapters(count) {
  if (!activeMangaDetail.value) return
  const ids = new Set()
  const total = activeMangaDetail.value.chapters.length
  const list = activeMangaDetail.value.chapters.slice(Math.max(0, total - count))
  list.forEach(c => ids.add(c.id))
  selectedChapterIDs.value = ids
}

async function submitDownloadTasks() {
  if (!activeMangaDetail.value || selectedChapterIDs.value.size === 0) return
  creatingTasks.value = true

  const selectedChs = activeMangaDetail.value.chapters.filter(c => selectedChapterIDs.value.has(c.id))
  const payload = {
    manga_id: activeMangaDetail.value.id,
    manga_title: activeMangaDetail.value.title,
    source: activeMangaDetail.value.source,
    format: selectedFormat.value,
    chapter_ids: selectedChs.map(c => c.id),
    chapter_names: selectedChs.map(c => c.title)
  }

  try {
    const res = await fetch('/api/tasks', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    })
    const json = await res.json()
    if (json.code === 0) {
      showDetailModal.value = false
      currentTab.value = 'tasks'
      loadTasks()
    }
  } catch (err) {
    console.error('Submit tasks error:', err)
  } finally {
    creatingTasks.value = false
  }
}

async function subscribeCurrentManga() {
  if (!activeMangaDetail.value) return
  const lastCh = activeMangaDetail.value.chapters[activeMangaDetail.value.chapters.length - 1]
  const payload = {
    manga_id: activeMangaDetail.value.id,
    manga_title: activeMangaDetail.value.title,
    source: activeMangaDetail.value.source,
    format: selectedFormat.value,
    last_chapter_id: lastCh ? lastCh.id : '',
    last_chapter_title: lastCh ? lastCh.title : '',
    auto_download: true
  }

  try {
    await fetch('/api/subscriptions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    })
    alert('已成功加入追更订阅！')
    loadSubscriptions()
  } catch (err) {
    console.error('Subscribe error:', err)
  }
}

async function loadTasks() {
  try {
    const res = await fetch('/api/tasks')
    const json = await res.json()
    if (json.code === 0) {
      tasks.value = json.data.map(t => ({ ...t, showLogs: false }))
    }
  } catch (err) {
    console.error('Load tasks error:', err)
  }
}

async function pauseTask(id) {
  await fetch(`/api/tasks/${id}/pause`, { method: 'POST' })
  loadTasks()
}

async function resumeTask(id) {
  await fetch(`/api/tasks/${id}/resume`, { method: 'POST' })
  loadTasks()
}

async function retryTask(id) {
  await fetch(`/api/tasks/${id}/retry`, { method: 'POST' })
  loadTasks()
}

async function deleteTask(id) {
  await fetch(`/api/tasks/${id}`, { method: 'DELETE' })
  loadTasks()
}

async function clearFinishedTasks() {
  await fetch('/api/tasks', { method: 'DELETE' })
  loadTasks()
}

async function loadSubscriptions() {
  try {
    const res = await fetch('/api/subscriptions')
    const json = await res.json()
    if (json.code === 0) {
      subscriptions.value = json.data
    }
  } catch (err) {
    console.error('Load subs error:', err)
  }
}

async function saveSubscription(sub) {
  await fetch('/api/subscriptions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(sub)
  })
}

async function deleteSubscription(id) {
  await fetch(`/api/subscriptions/${id}`, { method: 'DELETE' })
  loadSubscriptions()
}

async function checkUpdatesNow() {
  checkingUpdates.value = true
  try {
    const res = await fetch('/api/subscriptions/check', { method: 'POST' })
    const json = await res.json()
    alert(json.message || '检查完成！')
    loadSubscriptions()
    loadTasks()
  } catch (err) {
    console.error('Check update error:', err)
  } finally {
    checkingUpdates.value = false
  }
}

async function loadDownloadsList() {
  try {
    const res = await fetch('/api/downloads/list')
    const json = await res.json()
    if (json.code === 0) {
      downloadFiles.value = json.data
    }
  } catch (err) {
    console.error('Load downloads error:', err)
  }
}

async function loadConfig() {
  try {
    const res = await fetch('/api/config')
    const json = await res.json()
    if (json.code === 0) {
      appConfig.value = json.data
    }
  } catch (err) {
    console.error('Load config error:', err)
  }
}

async function saveAppConfig() {
  savingConfig.value = true
  try {
    const res = await fetch('/api/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(appConfig.value)
    })
    const json = await res.json()
    if (json.code === 0) {
      alert('配置已成功保存！')
    }
  } catch (err) {
    console.error('Save config error:', err)
  } finally {
    savingConfig.value = false
  }
}

function formatBytes(bytes) {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function setupSSE() {
  if (sseSource) sseSource.close()
  sseSource = new EventSource('/api/tasks/events')

  sseSource.addEventListener('task_update', (e) => {
    try {
      const updatedTask = JSON.parse(e.data)
      const idx = tasks.value.findIndex(t => t.id === updatedTask.id)
      if (idx >= 0) {
        const prevShow = tasks.value[idx].showLogs
        tasks.value[idx] = { ...updatedTask, showLogs: prevShow }
      } else {
        tasks.value.unshift({ ...updatedTask, showLogs: false })
      }
    } catch (err) {
      console.error('SSE parse error:', err)
    }
  })
}

onMounted(() => {
  loadHomeData()
  runSpeedTest()
  loadTasks()
  loadSubscriptions()
  loadConfig()
  setupSSE()
})

onUnmounted(() => {
  if (sseSource) sseSource.close()
})
</script>

<style scoped>
.app-container {
  max-width: 1280px;
  margin: 0 auto;
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* Header */
.bh-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 20px;
  padding: 20px 24px;
  background-color: var(--bh-white);
  border: var(--bh-border);
  box-shadow: var(--bh-shadow);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.bh-logo {
  display: flex;
  align-items: center;
  gap: 6px;
}

.shape-circle {
  width: 22px;
  height: 22px;
  background-color: var(--bh-red);
  border-radius: 50%;
  border: 2px solid var(--bh-black);
}

.shape-square {
  width: 20px;
  height: 20px;
  background-color: var(--bh-blue);
  border: 2px solid var(--bh-black);
}

.shape-triangle {
  width: 0;
  height: 0;
  border-left: 12px solid transparent;
  border-right: 12px solid transparent;
  border-bottom: 22px solid var(--bh-yellow);
}

.main-title {
  font-size: 24px;
  letter-spacing: -1px;
  line-height: 1.1;
}

.sub-title {
  font-size: 11px;
  font-weight: 700;
  color: #666;
  font-family: 'JetBrains Mono', monospace;
}

/* Header Right Single Speed Test Button */
.header-right {
  display: flex;
  align-items: center;
}

.speed-action-btn {
  font-weight: 700;
  font-size: 12px;
  padding: 8px 16px;
  box-shadow: 2px 2px 0px var(--bh-black);
}

/* Bauhaus Scrolling Marquee Decorative Ticker Bar */
.marquee-ticker-bar {
  display: flex;
  align-items: center;
  background: var(--bh-yellow);
  border: var(--bh-border);
  overflow: hidden;
  box-shadow: var(--bh-shadow-sm);
  height: 38px;
}

.ticker-label {
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--bh-black);
  color: var(--bh-white);
  padding: 0 14px;
  height: 100%;
  font-size: 11px;
  font-weight: 700;
  white-space: nowrap;
  z-index: 2;
  border-right: 2px solid var(--bh-black);
}

.ticker-dot {
  width: 8px;
  height: 8px;
  background-color: #4CAF50;
  border-radius: 50%;
  animation: pulse-green 1.5s infinite;
}

@keyframes pulse-green {
  0% { opacity: 0.4; }
  50% { opacity: 1; }
  100% { opacity: 0.4; }
}

.marquee-track {
  flex: 1;
  overflow: hidden;
  white-space: nowrap;
  position: relative;
  display: flex;
}

.marquee-content {
  display: inline-flex;
  align-items: center;
  gap: 24px;
  animation: ticker-scroll 35s linear infinite;
  padding-left: 20px;
}

.marquee-ticker-bar:hover .marquee-content {
  animation-play-state: paused;
}

@keyframes ticker-scroll {
  0% { transform: translateX(0); }
  100% { transform: translateX(-50%); }
}

.marquee-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 700;
  color: var(--bh-black);
}

.marquee-item .m-icon {
  font-size: 9px;
  color: var(--bh-red);
}

.marquee-item .m-latency {
  background: var(--bh-black);
  color: var(--bh-white);
  padding: 1px 6px;
  font-size: 11px;
  border-radius: 2px;
}

.marquee-item.is-fastest .m-latency {
  background: var(--bh-red);
}

.marquee-item .m-latency.offline {
  background: #757575;
}

.marquee-item .m-fastest-tag {
  background: var(--bh-red);
  color: var(--bh-white);
  padding: 1px 4px;
  font-size: 9px;
  font-family: 'JetBrains Mono', monospace;
}

.marquee-item.sep {
  font-size: 11px;
  opacity: 0.85;
  color: #222;
}

/* Navigation Tabs */
.bh-nav {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 12px;
}

.nav-tab {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  background-color: var(--bh-white);
  border: var(--bh-border);
  box-shadow: var(--bh-shadow-sm);
  cursor: pointer;
  transition: all 0.1s ease;
  position: relative;
  text-align: left;
}

.nav-tab:hover {
  transform: translate(-1px, -1px);
  box-shadow: var(--bh-shadow);
}

.nav-tab.active {
  background-color: var(--bh-black);
  color: var(--bh-white);
  transform: translate(2px, 2px);
  box-shadow: none;
}

.tab-idx {
  font-family: 'JetBrains Mono', monospace;
  font-size: 15px;
  font-weight: 900;
  opacity: 0.45;
}

.nav-tab.active .tab-idx {
  opacity: 0.9;
  color: var(--bh-yellow);
}

.tab-content-wrap {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.tab-name-zh {
  font-size: 14px;
  font-weight: 700;
  line-height: 1.2;
}

.tab-name-en {
  font-size: 10px;
  font-weight: 700;
  color: #777;
  letter-spacing: 0.5px;
}

.nav-tab.active .tab-name-en {
  color: #bbb;
}

.tab-badge {
  background-color: var(--bh-red);
  color: var(--bh-white);
  padding: 2px 7px;
  font-size: 11px;
  font-weight: 700;
}

/* Main Section */
.tab-section {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

/* Home Hero */
.home-hero {
  padding: 28px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 20px;
  background: linear-gradient(135deg, #FFFFFF 0%, #F5F2EB 100%);
  border-left: 6px solid var(--bh-red);
}

.hero-text {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-width: 780px;
}

.hero-title {
  font-size: 22px;
  letter-spacing: -0.5px;
}

.hero-desc {
  font-size: 13px;
  color: #444;
  line-height: 1.6;
}

.hero-btn {
  padding: 14px 28px;
  font-size: 15px;
}

.home-section-block {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.section-title-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  padding-bottom: 8px;
  border-bottom: 2px solid var(--bh-black);
}

.title-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.block-indicator {
  width: 14px;
  height: 14px;
  border: 1.5px solid var(--bh-black);
}

.red-block { background-color: var(--bh-red); }
.yellow-block { background-color: var(--bh-yellow); }

.section-heading {
  font-size: 18px;
  letter-spacing: -0.5px;
}

.sec-sub {
  font-size: 11px;
  color: #666;
}

/* Search Hero */
.search-hero {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.search-top-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.sec-tag {
  font-size: 12px;
  font-weight: 700;
}

.source-filter-pills {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.filter-pill {
  padding: 4px 10px;
  background-color: var(--bh-gray);
  border: var(--bh-border-thin);
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.1s ease;
}

.filter-pill.active {
  background-color: var(--bh-blue);
  color: var(--bh-white);
}

.search-input-box {
  display: flex;
  gap: 10px;
}

.search-field {
  font-size: 16px;
  padding: 12px 16px;
}

.search-btn {
  padding: 0 28px;
  font-size: 15px;
}

/* Results & Manga Grid */
.results-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.manga-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 20px;
}

.manga-card {
  display: flex;
  flex-direction: column;
  cursor: pointer;
  overflow: hidden;
}

.manga-cover-wrap {
  position: relative;
  width: 100%;
  padding-top: 135%;
  background: #eee;
  border-bottom: var(--bh-border-thin);
}

.manga-cover {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.rank-badge {
  position: absolute;
  top: 8px;
  right: 8px;
  font-size: 11px;
  font-weight: 700;
  background: var(--bh-black);
  color: var(--bh-white);
  padding: 2px 6px;
  border: 1px solid #fff;
}

.rank-badge.top-three {
  background: var(--bh-red);
}

.source-tag {
  position: absolute;
  top: 8px;
  left: 8px;
  font-size: 10px;
  font-weight: 700;
  padding: 2px 6px;
  background: var(--bh-black);
  color: var(--bh-white);
  border: 1px solid var(--bh-white);
}

.source-tag.src-copymanga { background: var(--bh-blue); }
.source-tag.src-dm5 { background: var(--bh-red); }
.source-tag.src-mangabz { background: var(--bh-yellow); color: var(--bh-black); }
.source-tag.src-pica { background: #E91E63; color: var(--bh-white); }

.latency-tag {
  position: absolute;
  bottom: 8px;
  right: 8px;
  font-size: 10px;
  background: rgba(0,0,0,0.75);
  color: #fff;
  padding: 2px 5px;
}

.manga-info {
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
}

.manga-title {
  font-size: 15px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.manga-author, .manga-latest {
  font-size: 12px;
  color: #555;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.full-w-btn {
  width: 100%;
  margin-top: auto;
}

/* Tasks / Queue */
.queue-header {
  padding: 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
}

.queue-stats {
  display: flex;
  gap: 20px;
}

.stat-item {
  display: flex;
  flex-direction: column;
}

.stat-num {
  font-size: 24px;
  font-weight: 700;
  line-height: 1;
}

.stat-lbl {
  font-size: 11px;
  color: #666;
}

.active-stat .stat-num { color: var(--bh-blue); }
.completed-stat .stat-num { color: #2E7D32; }
.failed-stat .stat-num { color: var(--bh-red); }

.queue-actions {
  display: flex;
  gap: 10px;
}

.tasks-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.task-card {
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.task-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.task-title-group {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.format-badge {
  padding: 2px 6px;
  font-size: 11px;
  font-weight: 700;
  background: var(--bh-black);
  color: var(--bh-white);
}

.task-manga {
  font-size: 16px;
}

.task-chapter {
  font-size: 14px;
  color: #555;
  font-weight: 600;
}

.task-badges {
  display: flex;
  gap: 6px;
}

.status-badge-downloading { background: var(--bh-blue); color: #fff; }
.status-badge-completed { background: #2E7D32; color: #fff; }
.status-badge-failed { background: var(--bh-red); color: #fff; }
.status-badge-paused { background: var(--bh-yellow); color: #000; }
.status-badge-pending { background: var(--bh-gray); color: #000; }

.task-progress-wrap {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.progress-labels {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
}

.progress-pct {
  font-weight: 700;
}

.completed-path {
  font-size: 12px;
  background: var(--bh-gray);
  padding: 6px 10px;
  border-left: 3px solid #2E7D32;
}

.error-banner {
  font-size: 12px;
  background: #FFEBEE;
  color: var(--bh-red);
  padding: 6px 10px;
  border-left: 3px solid var(--bh-red);
}

.task-logs-container {
  background: #FAFAFA;
  border: 1px solid #ddd;
}

.logs-header {
  padding: 8px 12px;
  cursor: pointer;
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: var(--bh-gray);
  font-size: 11px;
  font-weight: 700;
}

.latest-log-snippet {
  color: #555;
  font-weight: normal;
  max-width: 50%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.logs-body {
  padding: 10px 14px;
  max-height: 140px;
  overflow-y: auto;
  font-size: 12px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  background: #1e1e1e;
  color: #4AF626;
}

.task-controls {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

/* Modal */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  background: rgba(0, 0, 0, 0.65);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
}

.modal-content {
  width: 100%;
  max-width: 900px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  background: var(--bh-white);
  overflow: hidden;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: var(--bh-border);
  background: var(--bh-yellow);
}

.modal-title-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
}

.modal-close-btn {
  background: none;
  border: var(--bh-border-thin);
  font-size: 18px;
  font-weight: 700;
  cursor: pointer;
  padding: 4px 10px;
  background: var(--bh-white);
}

.modal-body {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow-y: auto;
}

.detail-top-card {
  display: flex;
  gap: 16px;
  background: var(--bh-gray);
  padding: 14px;
  border: var(--bh-border-thin);
}

.detail-cover {
  width: 100px;
  height: 140px;
  object-fit: cover;
  border: var(--bh-border-thin);
}

.detail-meta {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
}

.meta-desc {
  font-size: 12px;
  color: #444;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.chapter-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding: 10px 14px;
  background: var(--bh-gray);
  border: var(--bh-border-thin);
}

.format-picker {
  display: flex;
  align-items: center;
  gap: 8px;
}

.toolbar-lbl {
  font-size: 11px;
  font-weight: 700;
}

.format-tabs {
  display: flex;
  gap: 4px;
}

.fmt-btn {
  padding: 4px 10px;
  font-size: 12px;
  font-weight: 700;
  background: var(--bh-white);
  border: var(--bh-border-thin);
  cursor: pointer;
}

.fmt-btn.active {
  background: var(--bh-red);
  color: var(--bh-white);
}

.select-tools {
  display: flex;
  gap: 6px;
}

.latest-ch-badge {
  color: var(--bh-red);
  font-weight: 700;
}

.chapter-filter-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.ch-filter-input {
  flex: 1;
  padding: 6px 12px;
  font-size: 13px;
}

.ch-filter-count {
  font-size: 12px;
  color: #666;
  white-space: nowrap;
}

.chapters-grid-scroll {
  max-height: 280px;
  overflow-y: auto;
  border: var(--bh-border-thin);
  padding: 12px;
  background: #FAFAFA;
}

.chapters-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
  gap: 8px;
}

.chapter-tile {
  padding: 8px 10px;
  background: var(--bh-white);
  border: 1px solid #999;
  font-size: 12px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 6px;
  user-select: none;
}

.chapter-tile:hover {
  border-color: var(--bh-black);
}

.chapter-tile.selected {
  background: var(--bh-black);
  color: var(--bh-white);
  border-color: var(--bh-black);
}

.ch-name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

.ch-trial-tag {
  font-size: 9px;
  font-weight: 700;
  background-color: var(--bh-yellow);
  color: var(--bh-black);
  padding: 1px 4px;
  border-radius: 2px;
  font-family: 'JetBrains Mono', monospace;
}

.ch-full-tag {
  font-size: 9px;
  font-weight: 700;
  background-color: #2E7D32;
  color: var(--bh-white);
  padding: 1px 4px;
  border-radius: 2px;
  font-family: 'JetBrains Mono', monospace;
}

.chapter-group-tabs {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  padding-bottom: 4px;
}

.grp-tab-btn {
  padding: 6px 14px;
  background: var(--bh-white);
  border: var(--bh-border-thin);
  cursor: pointer;
  font-weight: 700;
  font-size: 12px;
  font-family: 'JetBrains Mono', monospace;
  transition: all 0.15s ease;
}

.grp-tab-btn:hover {
  background: var(--bh-gray);
  border-color: var(--bh-black);
}

.grp-tab-btn.active {
  background: var(--bh-black);
  color: var(--bh-white);
  border-color: var(--bh-black);
  box-shadow: 2px 2px 0px var(--bh-yellow);
}

.grp-count {
  font-size: 11px;
  opacity: 0.85;
}

.ch-vol-tag {
  font-size: 9px;
  font-weight: 700;
  background-color: #673AB7;
  color: var(--bh-white);
  padding: 1px 4px;
  border-radius: 2px;
  font-family: 'JetBrains Mono', monospace;
  white-space: nowrap;
}

.ch-extra-tag {
  font-size: 9px;
  font-weight: 700;
  background-color: #00897B;
  color: var(--bh-white);
  padding: 1px 4px;
  border-radius: 2px;
  font-family: 'JetBrains Mono', monospace;
  white-space: nowrap;
}

.is-volume-tile {
  border-color: #673AB7;
}

.empty-chapters-card {
  display: flex;
  gap: 16px;
  align-items: center;
  background: #FFF8E1;
  border: 2px solid var(--bh-yellow);
  padding: 18px;
}

.empty-alert-icon {
  font-size: 32px;
}

.empty-alert-content {
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: flex-start;
}

.empty-alert-desc {
  font-size: 13px;
  color: #444;
  line-height: 1.5;
}

.modal-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-top: 10px;
  border-top: var(--bh-border-thin);
}

.selected-count {
  color: var(--bh-red);
  font-size: 16px;
}

.auto-fallback-note {
  font-size: 11px;
  color: #666;
}

/* Subscriptions & Files */
.subscriptions-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

.sub-card {
  padding: 16px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 12px;
}

.sub-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.files-table-wrap {
  padding: 16px;
  overflow-x: auto;
}

.files-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.files-table th, .files-table td {
  padding: 10px 14px;
  text-align: left;
  border-bottom: 1px solid #ddd;
}

.files-table th {
  background: var(--bh-gray);
  font-weight: 700;
}

/* Settings Form */
.settings-box {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-label {
  font-size: 12px;
  font-weight: 700;
}

.form-hint {
  font-size: 11px;
  color: #666;
}

.form-grid-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.auto-dl-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
}

/* Loading & Empty */
.loading-box, .empty-box {
  padding: 40px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.bh-loader {
  width: 32px;
  height: 32px;
  border: 4px solid var(--bh-black);
  border-top-color: var(--bh-red);
  border-radius: 50%;
  animation: bh-spin 0.8s linear infinite;
}

@keyframes bh-spin {
  to { transform: rotate(360deg); }
}

.empty-title {
  font-size: 18px;
  font-weight: 700;
  font-family: 'Space Grotesk', sans-serif;
}

.empty-desc {
  font-size: 13px;
  color: #666;
}
</style>

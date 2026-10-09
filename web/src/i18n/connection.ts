export const connectionZh = {
  connectionTitle:'本地 AI 接入',connectionHint:'在自己的工具中完成工作，通过 Judex CLI 连接团队，提交材料与进展。',
  connectionCLI:'下载 Judex CLI',connectionCLIHint:'下载与你的操作系统对应的工具包，解压后即可使用。',
  connectionWindows:'下载 Windows CLI',connectionMac:'下载 macOS CLI',connectionWindowsArch:'支持 x64 与 ARM64',connectionMacArch:'支持 Intel 与 Apple Silicon',
  connectionSkills:'下载 Skills',connectionSkillsHint:'独立技能包可放入 Codex 或 Claude Code 的技能目录；CLI 工具包也包含配套 Skills。',
  connectionDownloadSkills:'下载 Skills ZIP',connectionVersion:'版本 {version}',connectionLoading:'正在获取下载信息…',connectionUnavailable:'下载暂不可用，请稍后重试。',connectionRetry:'重新加载',
  connectionStart:'连接工作空间',connectionStepOne:'解压下载包并在解压目录打开终端。Windows 使用 judex.cmd，macOS 使用 judex；启动文件会自动选择对应架构。',
  connectionStepTwo:'执行登录命令，并在浏览器中选择允许访问的项目。',connectionStepThree:'安装 Skills，并将解压目录加入 PATH，让本地 AI 可以调用 judex 提交材料与进展。正式决定仍由你在浏览器中确认。',
  connectionCopy:'复制命令',connectionCopied:'命令已复制',connectionCopyFailed:'复制失败，请手动选择命令。',connectionInstall:'选择你的 AI 工具，执行对应的安装命令',
  connectionTasks:'查看我的任务',connectionBoundary:'实际工作在本地完成，平台记录、分析并提出建议。',connectionWindowsCommand:'Windows PowerShell',connectionMacCommand:'macOS 终端',
};
export const connectionEn:Record<keyof typeof connectionZh,string> = {
  connectionTitle:'Local AI connection',connectionHint:'Work in your own tools. Connect to your team with Judex CLI and submit materials and progress.',
  connectionCLI:'Download Judex CLI',connectionCLIHint:'Choose your operating system and extract the archive to get started.',
  connectionWindows:'Download Windows CLI',connectionMac:'Download macOS CLI',connectionWindowsArch:'Supports x64 and ARM64',connectionMacArch:'Supports Intel and Apple Silicon',
  connectionSkills:'Download Skills',connectionSkillsHint:'Copy the standalone skill into your Codex or Claude Code skill directory. The CLI archives also include matching Skills.',
  connectionDownloadSkills:'Download Skills ZIP',connectionVersion:'Version {version}',connectionLoading:'Loading download information…',connectionUnavailable:'Downloads are temporarily unavailable. Please try again.',connectionRetry:'Reload',
  connectionStart:'Connect your workspace',connectionStepOne:'Extract the archive and open a terminal in its folder. Use judex.cmd on Windows or judex on macOS. The launcher selects the correct architecture.',
  connectionStepTwo:'Run the sign-in command and choose the allowed projects in your browser.',connectionStepThree:'Install Skills and add the extracted folder to PATH so your local AI can call judex to submit materials and progress. Confirm formal decisions in the browser.',
  connectionCopy:'Copy command',connectionCopied:'Command copied',connectionCopyFailed:'Copy failed. Select the command manually.',connectionInstall:'Choose your AI tool and run its install command',
  connectionTasks:'Open my tasks',connectionBoundary:'Work stays local. The platform records, analyzes and offers suggestions.',connectionWindowsCommand:'Windows PowerShell',connectionMacCommand:'macOS terminal',
};

import { useMemo, useState } from "react";
import EditIcon from "@mui/icons-material/Edit";
import MailOutlineIcon from "@mui/icons-material/MailOutline";
import { Fab, Box, useMediaQuery, useTheme } from "@mui/material";
import { Outlet, useMatch } from "react-router-dom";
import AppLayout from "../moles/AppLayout";
import ThreadList from "../moles/ThreadList";
import { useData } from "../../nonview/core/DataContext";
import { useCompose } from "../moles/ComposeProvider";
import EmptyState from "../atoms/EmptyState";

function MailPage() {
  const theme = useTheme();
  const isDesktop = useMediaQuery(theme.breakpoints.up("md"));
  const matchThread = useMatch("/thread/:threadId");
  const threadId = matchThread?.params?.threadId;

  const {
    emailThreads,
    loading,
    refreshActive,
    realtimeConnected,
    prefetchMessages,
    getThread,
  } = useData();
  const { openCompose } = useCompose();
  const [searchQuery, setSearchQuery] = useState("");

  const thread = threadId ? getThread(threadId) : undefined;

  const filteredThreads = useMemo(() => {
    if (!searchQuery) return emailThreads;
    const query = searchQuery.toLowerCase();
    return emailThreads.filter(
      (thread) =>
        thread.subject.toLowerCase().includes(query) ||
        thread.lastMessage.toLowerCase().includes(query) ||
        thread.participants.some((p) => p.name.toLowerCase().includes(query)),
    );
  }, [emailThreads, searchQuery]);

  const list = (
    <ThreadList
      threads={filteredThreads}
      loading={loading}
      emptyMessage={searchQuery ? "No emails match your search" : "No emails yet"}
      onRefresh={searchQuery ? undefined : refreshActive}
      live={realtimeConnected}
      onPrefetch={prefetchMessages}
      selectedThreadId={threadId}
    />
  );

  return (
    <AppLayout
      title={!isDesktop && matchThread ? (thread?.subject || "Thread") : "Quicksilver"}
      titleIcon={(!isDesktop && matchThread) ? null : MailOutlineIcon}
      showSearch={!matchThread || isDesktop}
      onSearch={setSearchQuery}
      actions={isDesktop ? [{ icon: EditIcon, label: "Compose", onClick: () => openCompose() }] : []}
    >
      {isDesktop ? (
        <Box sx={{ display: "flex", height: "100%" }}>
          <Box sx={{ width: 350, borderRight: 1, borderColor: "divider", height: "100%", flexShrink: 0 }}>
            {list}
          </Box>
          <Box sx={{ flex: 1, height: "100%", overflow: "hidden" }}>
            {matchThread ? <Outlet /> : <EmptyState title="Select an email to read" />}
          </Box>
        </Box>
      ) : matchThread ? (
        <Outlet />
      ) : (
        list
      )}
      {!isDesktop && !matchThread && (
        <Fab
          color="primary"
          aria-label="compose"
          onClick={() => openCompose()}
          sx={{
            position: "fixed",
            bottom: 24,
            right: 24,
          }}
        >
          <EditIcon />
        </Fab>
      )}
    </AppLayout>
  );
}

export default MailPage;

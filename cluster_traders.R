library(data.table)
library(dplyr)
library(ggplot2)

orders <- fread("data/orders.csv")

# Inspect the first few rows
print(head(orders, 10))

orders[, side := ifelse(grepl("^BUY", OrderType, ignore.case=TRUE), "BUY",
                 ifelse(grepl("^SELL", OrderType, ignore.case=TRUE), "SELL", NA_character_))]

features <- orders[, .(
  total_orders  = .N,
  total_new     = sum(Action == "EVENT_NEW"),
  total_cancel  = sum(Action == "EVENT_CANCEL"),
  buys          = sum(side == "BUY", na.rm=TRUE),
  sells         = sum(side == "SELL", na.rm=TRUE),
  avg_qty       = mean(as.numeric(Quantity), na.rm=TRUE),
  avg_price     = mean(as.numeric(Price), na.rm=TRUE)
), by=SubaccountID]

features[, cancel_ratio := total_cancel / pmax(total_new, 1)]
features[, buy_sell_ratio := buys / pmax(sells, 1)]

df[, .N, Action]

# (D) Construct a simple feature set by subaccount
#     For example:
#       - total # of placed orders
#       - total # of cancellations
#       - total # of executions
#       - total filled volume (sum of ExecQuantity)
#       - average placed Price
#       - average execution Price
#       - total Margin used

df_features <- df %>%
  filter(Subaccount != "") %>%   # ignore blank subaccounts if any
  group_by(Subaccount) %>%
  summarise(
    n_orders       = sum(Action == "PLACE_ORDER"),
    n_cancels      = sum(Action == "CANCEL_ORDER"),
    n_exec         = sum(Action == "EXECUTION"),
    total_fill_qty = sum(ExecQuantity, na.rm=TRUE),
    avg_price      = mean(Price, na.rm=TRUE),
    avg_exec_price = mean(ExecPrice, na.rm=TRUE),
    total_margin   = sum(Margin, na.rm=TRUE)
  )

print(head(df_features, 10))

# (E) Replace NAs with 0 (common for traders who never executed a trade, etc.)
df_features[is.na(df_features)] <- 0

# (F) Prepare the matrix for clustering
#     We'll cluster on numeric features only.
feature_cols <- c("n_orders", "n_cancels", "n_exec",
                  "total_fill_qty", "avg_price", "avg_exec_price", "total_margin")

X <- as.data.frame(df_features[, feature_cols])
# scale the numeric columns
X_scaled <- scale(X)

# (G) Run a simple K-means with, say, 5 clusters
set.seed(42)  # for reproducibility
k <- 5
km_res <- kmeans(X_scaled, centers=k)

# Inspect cluster sizes
cat("Cluster sizes:\n")
print(table(km_res$cluster))

# Add the cluster assignment to our df_features
df_features$cluster <- km_res$cluster

# (H) Basic plot
# We'll do a quick scatter of two principal components (PC1 & PC2) 
# to visualize clusters in 2D (or pick any two features).
pca_res <- prcomp(X_scaled, center=FALSE, scale.=FALSE)
df_pca <- data.frame(pca_res$x[,1:2], cluster = factor(km_res$cluster))
colnames(df_pca) <- c("PC1", "PC2", "cluster")

ggplot(df_pca, aes(x=PC1, y=PC2, color=cluster)) +
  geom_point(alpha=0.7, size=2) +
  theme_minimal() +
  ggtitle("K-means Clusters (PC1 vs PC2)")

# (I) Save results
# We can write the cluster-labeled data to CSV if desired
fwrite(df_features, "subaccount_clusters.csv")
cat("Wrote subaccount_clusters.csv with cluster assignments.\n")

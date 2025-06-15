# Free GitHub Actions Setup Guide

## 🆓 **100% Free Cloud Monitoring**

This guide shows you how to run your CSM monitoring tool **completely free** using GitHub Actions.

## ✅ **Benefits**
- **Completely FREE** (2,000 minutes/month - more than enough)
- **Always online** - runs in GitHub's cloud
- **No server management** required
- **Automatic updates** when you push code changes

## 🚀 **Setup Steps**

### 1. Push Your Code to GitHub
```bash
# Initialize git repository (if not already done)
git init
git add .
git commit -m "Initial CSM commit monitoring tool"

# Create a repository on GitHub and push
git remote add origin https://github.com/YOUR_USERNAME/csm.git
git push -u origin main
```

### 2. Add Discord Webhook as GitHub Secret
1. Go to your GitHub repository
2. Click **Settings** → **Secrets and variables** → **Actions**
3. Click **New repository secret**
4. Name: `DISCORD_WEBHOOK`
5. Value: `https://discord.com/api/webhooks/1383691064614977648/fapob3fWt9GfeM3qpQRTUSLh9teuclph4CNb5XyEyQmCs5KOdYNnWS4XAj-O9jmAQeZc`
6. Click **Add secret**

### 3. Enable GitHub Actions
- The workflow file is already created in `.github/workflows/monitor.yml`
- GitHub Actions will automatically start running every 15 minutes
- You can also trigger it manually from the Actions tab

### 4. Monitor Activity
- Go to **Actions** tab in your GitHub repository
- You'll see the monitoring workflow running every 15 minutes
- Check logs to see if new commits are detected

## 🔧 **How It Works**
1. **Every 15 minutes**: GitHub Actions runs your CSM tool
2. **Checks for commits**: Fetches latest commits from FAssets repository
3. **Sends notifications**: Posts to Discord if new commits found
4. **Updates tracking**: Commits the new last_commit SHA back to repository

## 💡 **Alternative Free Options**

### **Oracle Cloud Always Free**
- Forever free ARM VM
- 1-4 ARM CPU cores, 6-24 GB RAM
- No time limits

### **Google Cloud Free Tier**
- f1-micro instance (1 vCPU, 614 MB RAM)
- Free for 12 months

### **Railway.app**
- $5 free credits/month
- Easy deployment

## 🎯 **Recommendation**
**Start with GitHub Actions** - it's the easiest and most reliable free option for this use case! 